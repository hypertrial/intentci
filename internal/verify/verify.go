package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/internal/scheduler"
	"github.com/hypertrial/intentci/internal/trust"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Options configures a verification run.
type Options struct {
	Root     string
	Base     string
	Profile  string
	All      bool
	Trust    bool
	ChangeID string
	NoCache  bool
	Stdout   io.Writer
	Stderr   io.Writer
	Stream   bool
}

// Outcome is the verification result plus process exit code.
type Outcome struct {
	Result   *protocol.Result
	ExitCode int
}

// LastResultPath is where the latest JSON result is stored for explain.
func LastResultPath(root string) string {
	return filepath.Join(root, contract.DirName, "tmp", "last-result.json")
}

// Run executes the full verification pipeline.
func Run(ctx context.Context, opt Options) (*Outcome, error) {
	if opt.Stdout == nil {
		opt.Stdout = os.Stdout
	}
	if opt.Stderr == nil {
		opt.Stderr = os.Stderr
	}
	if opt.Profile == "" {
		opt.Profile = "full"
	}

	c, raw, err := contract.LoadFromRoot(opt.Root)
	if err != nil {
		return &Outcome{ExitCode: 20}, fmt.Errorf("%w", err)
	}
	if err := contract.Validate(c); err != nil {
		return &Outcome{ExitCode: 20}, err
	}

	var (
		change     *changespec.Spec
		changeRaw  []byte
		changeHash string
		findings   []protocol.ChangeFinding
	)
	if opt.ChangeID != "" {
		change, changeRaw, err = changespec.Load(opt.Root, opt.ChangeID)
		if err != nil {
			return &Outcome{ExitCode: 20}, err
		}
		if err := changespec.Validate(change, c); err != nil {
			return &Outcome{ExitCode: 20}, err
		}
		changeHash = changespec.Hash(changeRaw)
	}

	base := opt.Base
	if base == "" {
		base = c.Policy.DefaultBaseOr("origin/main")
	}

	state, err := git.Resolve(opt.Root, base)
	if err != nil {
		return &Outcome{ExitCode: 21}, err
	}

	if change != nil {
		// Compare against the merge-base tree (same commit as result.base_commit),
		// not the tip of the base ref, so diverged main history cannot skew findings.
		baseData, ok, _ := changespec.LoadBase(opt.Root, state.MergeBaseFull, change.ID)
		findings = changespec.DiffApproved(change.ID, baseData, ok, change, changeRaw)
	}

	impactOpt := impact.Options{
		All:     opt.All,
		Profile: opt.Profile,
	}
	if change != nil {
		impactOpt.ForceRequirementIDs = append([]string{}, change.AffectedRequirements...)
		impactOpt.ForceCheckIDs = append([]string{}, change.RequiredChecks...)
		for _, ac := range change.Acceptance {
			impactOpt.ExtraRequirements = append(impactOpt.ExtraRequirements, contract.Requirement{
				ID:        ac.ID,
				Type:      "behavior",
				Title:     ac.ID,
				Statement: ac.Statement,
				Status:    "approved",
				Severity:  ac.Severity,
				Verification: contract.Verification{
					Mode:   "all",
					Checks: ac.Verification.Checks,
				},
			})
		}
	}

	sel := impact.Resolve(c, state.ChangedFiles, impactOpt)
	contractHash := contract.Hash(raw)

	var store *cache.Store
	if !opt.NoCache {
		store, err = openCache("")
		if err != nil {
			return &Outcome{ExitCode: 30}, err
		}
	}

	var checkResults map[string]runner.Result
	if len(sel.CheckIDs) > 0 {
		if err := trust.Ensure(opt.Root, opt.Trust, os.Stdin, opt.Stderr); err != nil {
			return &Outcome{ExitCode: 21}, err
		}
		var outW, errW io.Writer
		if opt.Stream {
			outW = opt.Stdout
			errW = opt.Stderr
		}
		checkResults = scheduleChecks(ctx, c.CheckMap(), sel.CheckIDs, scheduler.Options{
			Dir:          opt.Root,
			MaxParallel:  c.Execution.MaxParallelOr(0),
			Stdout:       outW,
			Stderr:       errW,
			Cache:        store,
			NoCache:      opt.NoCache,
			ContractHash: contractHash,
			ChangeHash:   changeHash,
			EnvInclude:   c.Environment.Include,
		})
	} else {
		checkResults = map[string]runner.Result{}
	}

	reqResults := evidence.Assign(sel, checkResults, opt.Profile, c)
	status, exitCode := evidence.Overall(reqResults, c.Policy)

	executed := 0
	cached := 0
	checkList := make([]protocol.CheckResult, 0, len(checkResults))
	for _, id := range sel.CheckIDs {
		res, ok := checkResults[id]
		if !ok {
			continue
		}
		if res.FromCache {
			cached++
		} else if res.Status == protocol.CheckPass || res.Status == protocol.CheckFail || res.Status == protocol.CheckUnknown {
			executed++
		}
		checkList = append(checkList, protocol.CheckResult{
			ID:         id,
			Status:     res.Status,
			ExitCode:   res.ExitCode,
			DurationMS: res.DurationMS,
			Stdout:     runner.Truncate(res.Stdout, 32*1024),
			Stderr:     runner.Truncate(res.Stderr, 32*1024),
			Reason:     res.Reason,
			FromCache:  res.FromCache,
		})
	}

	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	runID := ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()

	var changeRef *protocol.ChangeSpecRef
	if change != nil {
		changeRef = &protocol.ChangeSpecRef{ID: change.ID, Hash: changeHash}
	}
	if findings == nil {
		findings = []protocol.ChangeFinding{}
	}

	summary := evidence.Summarize(reqResults, executed)
	summary.ChecksCached = cached

	result := &protocol.Result{
		SchemaVersion:    protocol.SchemaVersion,
		RunID:            runID,
		Status:           status,
		BaseCommit:       state.MergeBase,
		HeadCommit:       state.HeadCommit,
		ContractHash:     contractHash,
		WorkingTreeDirty: state.WorkingTreeDirty,
		Profile:          opt.Profile,
		ChangeSpec:       changeRef,
		Requirements:     reqResults,
		Checks:           checkList,
		Waivers:          []any{},
		ContractChanges:  []any{},
		ChangeFindings:   findings,
		Summary:          summary,
	}

	_ = persistLastResult(opt.Root, result)

	return &Outcome{Result: result, ExitCode: exitCode}, nil
}

var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile
var marshalIndent = json.MarshalIndent

func persistLastResult(root string, result *protocol.Result) error {
	path := LastResultPath(root)
	if err := mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := marshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, data, 0o644)
}

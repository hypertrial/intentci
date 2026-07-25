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
	"github.com/hypertrial/intentci/internal/contractdiff"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/internal/scheduler"
	"github.com/hypertrial/intentci/internal/semantic"
	"github.com/hypertrial/intentci/internal/trust"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Options configures a verification run.
type Options struct {
	Root              string
	Base              string
	Profile           string
	All               bool
	Trust             bool
	ChangeID          string
	NoCache           bool
	Attest            bool
	ShowSemanticInput bool
	Stdout            io.Writer
	Stderr            io.Writer
	Stream            bool
}

// Outcome is the verification result plus process exit code.
type Outcome struct {
	Result          *protocol.Result
	ExitCode        int
	AttestationPath string
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

	headContract, raw, err := contract.LoadFromRoot(opt.Root)
	if err != nil {
		return &Outcome{ExitCode: 20}, fmt.Errorf("%w", err)
	}
	if err := contract.Validate(headContract); err != nil {
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
		if err := changespec.Validate(change, headContract); err != nil {
			return &Outcome{ExitCode: 20}, err
		}
		changeHash = changespec.Hash(changeRaw)
	}

	base := opt.Base
	if base == "" {
		base = headContract.Policy.DefaultBaseOr("origin/main")
	}

	state, err := git.Resolve(opt.Root, base)
	if err != nil {
		return &Outcome{ExitCode: 21}, err
	}

	baseContract, _, baseOK, err := loadBaseContract(opt.Root, state.MergeBaseFull)
	if err != nil {
		return &Outcome{ExitCode: 20}, err
	}
	var contractChanges []protocol.ContractChange
	effective := headContract
	if baseOK {
		contractChanges = contractdiff.Diff(baseContract, headContract)
		effective = contractdiff.Effective(baseContract, headContract)
	}
	if contractChanges == nil {
		contractChanges = []protocol.ContractChange{}
	}

	if change != nil {
		baseData, ok, _ := changespec.LoadBase(opt.Root, state.MergeBaseFull, change.ID)
		findings = changespec.DiffApproved(change.ID, baseData, ok, change, changeRaw)
	}

	impactOpt := impact.Options{
		All:     opt.All,
		Profile: opt.Profile,
	}
	waived := map[string]protocol.Waiver{}
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
					Mode:     "all",
					Checks:   ac.Verification.Checks,
					Semantic: ac.Verification.Semantic,
				},
			})
		}
		// Waivers apply only from approved Change Specs (drafts may define them for later).
		if change.Status == "approved" {
			for _, w := range change.Waivers {
				waived[w.Requirement] = protocol.Waiver{
					ID:          w.ID,
					Requirement: w.Requirement,
					Reason:      w.Reason,
					Owner:       w.Owner,
					Approver:    w.Approver,
					Expires:     w.Expires,
				}
			}
		}
	}

	sel := impact.Resolve(effective, state.ChangedFiles, impactOpt)
	contractHash := contract.Hash(raw)

	// Privacy preview: build and print semantic input without running checks or providers.
	if opt.ShowSemanticInput {
		semOut, err := runSemantic(ctx, semantic.RunOptions{
			Root:              opt.Root,
			Profile:           opt.Profile,
			BaseCommit:        state.MergeBaseFull,
			HeadCommit:        state.HeadCommit,
			ChangedFiles:      state.ChangedFiles,
			Contract:          effective,
			Change:            change,
			Selection:         sel,
			CheckResults:      map[string]runner.Result{},
			ShowSemanticInput: true,
			Stdout:            opt.Stdout,
			TrustLocal:        true, // no local provider execution on preview
		})
		if err != nil {
			return &Outcome{ExitCode: 30}, err
		}
		if !semOut.ShowedInput {
			return &Outcome{ExitCode: 30}, fmt.Errorf("failed to render semantic input")
		}
		return &Outcome{ExitCode: 0}, nil
	}

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
		checkResults = scheduleChecks(ctx, effective.CheckMap(), sel.CheckIDs, scheduler.Options{
			Dir:          opt.Root,
			MaxParallel:  effective.Execution.MaxParallelOr(0),
			Stdout:       outW,
			Stderr:       errW,
			Cache:        store,
			NoCache:      opt.NoCache,
			ContractHash: contractHash,
			ChangeHash:   changeHash,
			EnvInclude:   effective.Environment.Include,
		})
	} else {
		checkResults = map[string]runner.Result{}
	}

	reqResults := evidence.Assign(sel, checkResults, opt.Profile, effective, waived)

	semOut, err := runSemantic(ctx, semantic.RunOptions{
		Root:         opt.Root,
		Profile:      opt.Profile,
		BaseCommit:   state.MergeBaseFull,
		HeadCommit:   state.HeadCommit,
		ChangedFiles: state.ChangedFiles,
		Contract:     effective,
		Change:       change,
		Selection:    sel,
		CheckResults: checkResults,
		Stdout:       opt.Stdout,
		TrustLocal:   opt.Trust,
		EnsureTrust: func() error {
			return trust.Ensure(opt.Root, opt.Trust, os.Stdin, opt.Stderr)
		},
	})
	if err != nil {
		return &Outcome{ExitCode: 30}, err
	}

	mergeOpt := semantic.MergeOptions{
		Policy:        effective.Policy.Semantic,
		Contract:      effective,
		SemanticModes: semantic.ModesFromSelection(sel),
	}
	if effective.Policy.Semantic.Enabled {
		if semOut.ProviderErr != nil {
			reqResults = semantic.MarkUnavailable(reqResults, mergeOpt, semOut.ProviderErr.Error())
		} else if len(semOut.Findings) > 0 {
			reqResults = semantic.Apply(reqResults, semOut.Findings, mergeOpt)
		}
	}

	status, exitCode := evidence.Overall(reqResults, effective.Policy)

	// Contract-approval gate: weakenings without type:contract force unverified.
	if len(contractChanges) > 0 {
		approvedContractChange := change != nil && change.Status == "approved" && change.Type == "contract"
		if !approvedContractChange && status == protocol.StatusPass {
			status = protocol.StatusUnverified
			exitCode = 11
		}
	}

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

	waiverList := make([]protocol.Waiver, 0, len(waived))
	for _, w := range waived {
		waiverList = append(waiverList, w)
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
		Waivers:          waiverList,
		ContractChanges:  contractChanges,
		ChangeFindings:   findings,
		Semantic:         semOut.SemanticRun,
		Summary:          summary,
	}

	_ = persistLastResult(opt.Root, result)

	out := &Outcome{Result: result, ExitCode: exitCode}
	if opt.Attest {
		if status != protocol.StatusPass || hasNonPassChecks(checkList) {
			// Do not write attestation on non-pass overall or when any check failed/unknown
			// (e.g. waiver-driven PASS with failing check records).
			return out, nil
		}
		att, err := buildAttestation(result, effective.CheckMap(), effective.Environment.Include)
		if err != nil {
			return &Outcome{Result: result, ExitCode: 30}, err
		}
		path, err := writeAttestation(opt.Root, att)
		if err != nil {
			return &Outcome{Result: result, ExitCode: 30}, err
		}
		out.AttestationPath = path
		fmt.Fprintf(opt.Stderr, "Wrote attestation: %s\n", path)
	}

	return out, nil
}

var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile
var marshalIndent = json.MarshalIndent

func hasNonPassChecks(checks []protocol.CheckResult) bool {
	for _, ch := range checks {
		switch ch.Status {
		case protocol.CheckPass, protocol.CheckSkipped, protocol.CheckCached:
			continue
		default:
			return true
		}
	}
	return false
}

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

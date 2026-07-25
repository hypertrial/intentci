package verify

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/oklog/ulid/v2"

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
	Stdout   io.Writer
	Stderr   io.Writer
	Stream   bool
}

// Outcome is the verification result plus process exit code.
type Outcome struct {
	Result   *protocol.Result
	ExitCode int
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

	base := opt.Base
	if base == "" {
		base = c.Policy.DefaultBaseOr("origin/main")
	}

	state, err := git.Resolve(opt.Root, base)
	if err != nil {
		return &Outcome{ExitCode: 21}, err
	}

	sel := impact.Resolve(c, state.ChangedFiles, impact.Options{
		All:     opt.All,
		Profile: opt.Profile,
	})

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
		checkResults = scheduler.Run(ctx, c.CheckMap(), sel.CheckIDs, scheduler.Options{
			Dir:         opt.Root,
			MaxParallel: c.Execution.MaxParallelOr(0),
			Stdout:      outW,
			Stderr:      errW,
		})
	} else {
		checkResults = map[string]runner.Result{}
	}

	reqResults := evidence.Assign(sel, checkResults, opt.Profile, c)
	status, exitCode := evidence.Overall(reqResults, c.Policy)

	executed := 0
	checkList := make([]protocol.CheckResult, 0, len(checkResults))
	for _, id := range sel.CheckIDs {
		res, ok := checkResults[id]
		if !ok {
			continue
		}
		if res.Status == protocol.CheckPass || res.Status == protocol.CheckFail || res.Status == protocol.CheckUnknown {
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
		})
	}

	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	runID := ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()

	result := &protocol.Result{
		SchemaVersion:    protocol.SchemaVersion,
		RunID:            runID,
		Status:           status,
		BaseCommit:       state.MergeBase,
		HeadCommit:       state.HeadCommit,
		ContractHash:     contract.Hash(raw),
		WorkingTreeDirty: state.WorkingTreeDirty,
		Profile:          opt.Profile,
		ChangeSpec:       nil,
		Requirements:     reqResults,
		Checks:           checkList,
		Waivers:          []any{},
		ContractChanges:  []any{},
		Summary:          evidence.Summarize(reqResults, executed),
	}

	return &Outcome{Result: result, ExitCode: exitCode}, nil
}

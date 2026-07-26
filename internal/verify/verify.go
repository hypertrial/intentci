package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/hypertrial/intentci/internal/compiler"
	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/executor"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

// Options configures verification.
type Options struct {
	Root          string
	Base          string
	Head          string
	All           bool
	Changed       bool
	RequirementID string
	ObligationID  string
	NoCache       bool
	Format        string
}

// Outcome is the verification result.
type Outcome struct {
	Bundle   *evidence.Bundle
	ExitCode int
}

// Run compiles, selects, executes, and persists a verification run.
func Run(ctx context.Context, opt Options) (*Outcome, error) {
	cfg, err := config.Load(opt.Root)
	if err != nil {
		return &Outcome{ExitCode: exitcode.CompileFailed}, err
	}
	comp, err := compiler.Compile(compiler.Options{
		Root: opt.Root, Config: cfg, RequirementID: opt.RequirementID, Strict: true,
	})
	if err != nil {
		return &Outcome{ExitCode: exitcode.CompileFailed}, err
	}

	base := opt.Base
	if base == "" {
		base = cfg.BaseRefOr("origin/main")
	}
	useAll := opt.All || opt.RequirementID != ""
	useChanged := opt.Changed || (!opt.All && opt.RequirementID == "")
	if opt.All {
		useChanged = false
		useAll = true
	}

	var state *git.State
	st, gerr := git.Resolve(opt.Root, base)
	if gerr != nil {
		if useAll && !useChanged {
			state = &git.State{Root: opt.Root, HeadCommit: "unknown", ChangedFiles: nil}
		} else {
			return &Outcome{ExitCode: exitcode.VerifierError}, gerr
		}
	} else {
		state = st
	}

	sel := impact.Select(comp.Document, impact.Options{
		All:           useAll && !useChanged,
		RequirementID: opt.RequirementID,
		ObligationID:  opt.ObligationID,
		ChangedFiles:  state.ChangedFiles,
	})

	store, err := newStore(opt.Root, cfg.Evidence.Directory)
	if err != nil {
		return &Outcome{ExitCode: exitcode.Internal}, err
	}
	runID := evidence.NewRunID()
	cacheDir := filepath.Join(config.Dir(opt.Root), "cache")

	cfgHash := hashConfig(cfg)
	_, reqResults := executor.Run(ctx, sel.Requirements, executor.Options{
		Root: opt.Root, Config: cfg, Registry: provider.DefaultRegistry(),
		BaseCommit: state.MergeBaseFull, HeadCommit: state.HeadCommit,
		ChangedFiles: state.ChangedFiles, RunID: runID, IRHash: comp.Document.Hash,
		NoCache: opt.NoCache, CacheDir: cacheDir,
	})
	run := verdict.AggregateRun(reqResults)

	bundle := &evidence.Bundle{
		RunID: runID, CreatedAt: time.Now().UTC(), Root: opt.Root,
		BaseCommit: state.MergeBaseFull, HeadCommit: state.HeadCommit,
		ConfigHash: cfgHash, IRHash: comp.Document.Hash, Document: comp.Document,
		Run: run, Unmapped: sel.Unmapped,
	}
	_ = useChanged // unmapped files are reported on the bundle for callers/CI
	if err := persistBundle(store, bundle); err != nil {
		return &Outcome{Bundle: bundle, ExitCode: exitcode.Internal}, err
	}
	return &Outcome{Bundle: bundle, ExitCode: verdict.ExitCode(run.Verdict)}, nil
}

func hashConfig(cfg *config.Config) string {
	b, _ := json.Marshal(cfg)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

var persistBundle = func(store *evidence.Store, b *evidence.Bundle) error {
	return store.WriteBundle(b)
}

var newStore = evidence.NewStore

package verify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hypertrial/intentci/internal/compiler"
	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/evidence"
	"github.com/hypertrial/intentci/internal/executor"
	"github.com/hypertrial/intentci/internal/exitcode"
	"github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/internal/verdict"
	appschema "github.com/hypertrial/intentci/pkg/schema"
)

// Options configures verification.
type Options struct {
	Root           string
	Base           string
	Head           string
	All            bool
	Changed        bool
	RequirementID  string
	ObligationID   string
	ProviderID     string
	MaxParallel    int
	MaxParallelSet bool
	FailFast       bool
	FailFastSet    bool
	NoGit          bool
	NoCache        bool
	Format         string
	Config         *config.Config
	Document       *ir.Document
	RunID          string
	AttemptID      string
	AttemptOnly    bool
}

// Outcome is the verification result.
type Outcome struct {
	Bundle   *evidence.Bundle
	ExitCode int
}

// Run compiles, selects, executes, and persists a verification run.
func Run(ctx context.Context, opt Options) (*Outcome, error) {
	cfg := opt.Config
	if cfg == nil {
		var err error
		cfg, err = config.Load(opt.Root)
		if err != nil {
			return &Outcome{ExitCode: exitcode.CompileFailed}, err
		}
	}
	document := opt.Document
	if document == nil {
		comp, err := compiler.Compile(compiler.Options{
			Root: opt.Root, Config: cfg, Strict: true,
		})
		if err != nil {
			return &Outcome{ExitCode: exitcode.CompileFailed}, err
		}
		document = comp.Document
	} else if err := appschema.Validate("ir", document); err != nil {
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

	if err := validateSelectors(document, opt); err != nil {
		return &Outcome{ExitCode: exitcode.Usage}, err
	}
	if opt.MaxParallelSet {
		cfg.Verification.MaxParallel = opt.MaxParallel
	}
	if opt.FailFastSet {
		cfg.Verification.FailFast = opt.FailFast
	}

	var state *git.State
	if opt.NoGit {
		if useChanged {
			return &Outcome{ExitCode: exitcode.Usage}, fmt.Errorf("--no-git requires --all or an explicit requirement")
		}
		state = &git.State{Root: opt.Root, HeadCommit: "unknown", DiffHash: ir.HashBytes(nil)}
	} else {
		st, gerr := git.ResolveWithOptions(opt.Root, git.ResolveOptions{
			BaseRef: base, HeadRef: opt.Head, IncludeUntracked: cfg.ChangeImpact.IncludeUntracked,
		})
		if gerr != nil {
			if useAll && !useChanged {
				state = &git.State{Root: opt.Root, HeadCommit: "unknown", ChangedFiles: nil, DiffHash: ir.HashBytes(nil)}
			} else {
				return &Outcome{ExitCode: exitcode.VerifierError}, gerr
			}
		} else {
			state = st
		}
		if cfg.Verification.RequireCleanWorktree && state.WorkingTreeDirty {
			return &Outcome{ExitCode: exitcode.VerifierError}, fmt.Errorf("working tree must be clean")
		}
	}

	sel := impact.Select(document, impact.Options{
		All:                     useAll && !useChanged,
		RequirementID:           opt.RequirementID,
		ObligationID:            opt.ObligationID,
		ChangedFiles:            state.ChangedFiles,
		GlobalPaths:             cfg.ChangeImpact.GlobalPaths,
		RunUnmappedRequirements: cfg.ChangeImpact.RunUnmappedRequirements,
	})
	plan, _ := ir.BuildVerificationPlan(document, sel.Requirements)

	store, err := newStore(opt.Root, cfg.Evidence.Directory)
	if err != nil {
		return &Outcome{ExitCode: exitcode.Internal}, err
	}
	runID := opt.RunID
	if runID == "" {
		runID = evidence.NewRunID()
	}
	attemptID := opt.AttemptID
	if attemptID == "" {
		attemptID = "attempt-001"
	}
	cacheDir := filepath.Join(config.Dir(opt.Root), "cache")

	cfgHash := hashConfig(cfg)
	providerResults, reqResults := executor.Run(ctx, sel.Requirements, executor.Options{
		Root: opt.Root, Config: cfg, Registry: provider.DefaultRegistry(),
		BaseCommit: state.MergeBaseFull, HeadCommit: state.HeadCommit,
		DiffHash: state.DiffHash, ChangedFiles: state.ChangedFiles,
		Changes: providerChanges(state.Changes),
		RunID:   runID, AttemptID: attemptID,
		EvidenceDir: filepath.Join(store.Dir(runID), "attempts", attemptID, "artifacts"),
		IRHash:      document.Hash, PlanHash: plan.Hash, ProviderID: opt.ProviderID,
		NoCache: opt.NoCache, CacheDir: cacheDir,
	})
	run := verdict.AggregateRun(reqResults)
	if cfg.ChangeImpact.FailOnUnmapped && len(sel.Unmapped) > 0 {
		run.Verdict = verdict.Fail
	}
	bundle := &evidence.Bundle{
		RunID: runID, AttemptID: attemptID, CreatedAt: time.Now().UTC(), Root: opt.Root,
		BaseCommit: state.MergeBaseFull, HeadCommit: state.HeadCommit,
		ConfigHash: cfgHash, IRHash: document.Hash, Document: document,
		Run: run, Unmapped: sel.Unmapped, ProviderLogs: flattenProviderResults(providerResults),
		RepositoryState: state, VerificationPlan: plan,
	}
	if ctx.Err() != nil {
		bundle.Interrupted = true
		bundle.Run.Verdict = verdict.Error
	}
	_ = useChanged // unmapped files are reported on the bundle for callers/CI
	store.RedactPatterns = append([]string{}, cfg.Evidence.Redact.Environment...)
	persist := persistBundle
	if opt.AttemptOnly {
		persist = func(store *evidence.Store, bundle *evidence.Bundle) error {
			return store.WriteAttempt(bundle)
		}
	}
	if err := persist(store, bundle); err != nil {
		return &Outcome{Bundle: bundle, ExitCode: exitcode.Internal}, err
	}
	if bundle.Interrupted {
		return &Outcome{Bundle: bundle, ExitCode: exitcode.VerifierError}, nil
	}
	if hasSecurityViolation(providerResults) {
		return &Outcome{Bundle: bundle, ExitCode: exitcode.SecurityBoundary}, nil
	}
	return &Outcome{Bundle: bundle, ExitCode: verdict.ExitCodeConfigured(run.Verdict, cfg.CI.FailOn)}, nil
}

func providerChanges(changes []git.Change) []provider.Change {
	output := make([]provider.Change, 0, len(changes))
	for _, change := range changes {
		output = append(output, provider.Change{
			Path: change.Path, OldPath: change.OldPath, Status: change.Status,
			Additions: change.Additions, Deletions: change.Deletions, Binary: change.Binary,
			OldMode: change.OldMode, NewMode: change.NewMode,
		})
	}
	return output
}

// FinalizeBundle emits reports, the manifest, and the final verdict for a run
// whose immutable attempts have already been persisted.
func FinalizeBundle(store *evidence.Store, bundle *evidence.Bundle) error {
	for _, item := range []struct {
		format string
		name   string
	}{
		{format: "text", name: "report.txt"},
		{format: "json", name: "report.json"},
		{format: "junit", name: "report.junit.xml"},
	} {
		var output bytes.Buffer
		if err := report.Write(&output, item.format, bundle); err != nil {
			return err
		}
		if err := store.WriteReport(bundle.RunID, item.name, output.Bytes()); err != nil {
			return err
		}
	}
	return store.Finalize(bundle)
}

func validateSelectors(document *ir.Document, options Options) error {
	if options.RequirementID != "" && document.RequirementByID(options.RequirementID) == nil {
		return fmt.Errorf("requirement %q not found", options.RequirementID)
	}
	obligationFound := options.ObligationID == ""
	providerFound := options.ProviderID == ""
	for _, requirement := range document.Requirements {
		if options.RequirementID != "" && requirement.ID != options.RequirementID {
			continue
		}
		for _, obligation := range requirement.Obligations {
			if obligation.ID == options.ObligationID {
				obligationFound = true
			}
			var walk func(ir.VerifyNode)
			walk = func(node ir.VerifyNode) {
				if node.Provider != nil && node.Provider.ID == options.ProviderID {
					providerFound = true
				}
				for _, child := range node.All {
					walk(child)
				}
				for _, child := range node.Any {
					walk(child)
				}
				if node.Not != nil {
					walk(*node.Not)
				}
			}
			walk(obligation.Verify)
		}
	}
	if !obligationFound {
		return fmt.Errorf("obligation %q not found", options.ObligationID)
	}
	if !providerFound {
		return fmt.Errorf("provider %q not found", options.ProviderID)
	}
	return nil
}

func flattenProviderResults(results map[string]executor.LeafResult) map[string]provider.Result {
	flattened := map[string]provider.Result{}
	for requirement, leaves := range results {
		for key, result := range leaves {
			flattened[requirement+"/"+key] = result
		}
	}
	return flattened
}

func hasSecurityViolation(results map[string]executor.LeafResult) bool {
	for _, leaves := range results {
		for _, result := range leaves {
			if result.SecurityViolation {
				return true
			}
		}
	}
	return false
}

func hashConfig(cfg *config.Config) string {
	b, _ := json.Marshal(cfg)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

var persistBundle = persistRunBundle

func persistRunBundle(store *evidence.Store, bundle *evidence.Bundle) error {
	if err := store.WriteAttempt(bundle); err != nil {
		return err
	}
	return FinalizeBundle(store, bundle)
}

var newStore = evidence.NewStore

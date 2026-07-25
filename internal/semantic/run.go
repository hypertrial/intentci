package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/hypertrial/intentci/internal/changespec"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

var (
	buildRequest = BuildRequest
	newProvider  = NewProvider
	encodeJSON   = func(w io.Writer, v any) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
)

// RunOptions configures a semantic verification pass.
type RunOptions struct {
	Root              string
	Profile           string
	BaseCommit        string
	HeadCommit        string
	Contract          *contract.Contract
	Change            *changespec.Spec
	Selection         impact.Selection
	ChangedFiles      []string
	CheckResults      map[string]runner.Result
	ShowSemanticInput bool
	Stdout            io.Writer
	// Provider overrides NewProvider in tests.
	Provider Provider
	// TrustLocal is true when the repository is trusted for local executables.
	TrustLocal bool
	// EnsureTrust is called before local provider execution when TrustLocal is false.
	EnsureTrust func() error
}

// RunResult is the outcome of an optional semantic pass.
type RunResult struct {
	Request      Request
	Findings     []Finding
	SemanticRun  *protocol.SemanticRun
	ShowedInput  bool
	ProviderErr  error
}

// Run builds input, optionally prints it, and invokes the configured provider.
func Run(ctx context.Context, opt RunOptions) (RunResult, error) {
	policy := opt.Contract.Policy.Semantic

	req, err := buildRequest(BuildOptions{
		Root:         opt.Root,
		Profile:      opt.Profile,
		BaseCommit:   opt.BaseCommit,
		HeadCommit:   opt.HeadCommit,
		ChangedFiles: opt.ChangedFiles,
		Contract:     opt.Contract,
		Change:       opt.Change,
		Selection:    opt.Selection,
		CheckResults: opt.CheckResults,
		EnvInclude:   opt.Contract.Environment.Include,
	})
	if err != nil {
		return RunResult{}, err
	}

	if opt.ShowSemanticInput {
		if opt.Stdout == nil {
			return RunResult{}, fmt.Errorf("stdout is required for --show-semantic-input")
		}
		if err := encodeJSON(opt.Stdout, req); err != nil {
			return RunResult{}, err
		}
		return RunResult{
			Request:     req,
			ShowedInput: true,
			SemanticRun: &protocol.SemanticRun{
				Enabled:  policy.Enabled,
				Provider: providerType(policy),
				Skipped:  "show-semantic-input",
			},
		}, nil
	}

	if !policy.Enabled {
		return RunResult{
			Request: req,
			SemanticRun: &protocol.SemanticRun{
				Enabled:  false,
				Provider: "none",
				Skipped:  "disabled",
			},
		}, nil
	}
	if policy.Provider == nil {
		return unavailable(req, policy, "semantic provider is not configured"), nil
	}

	prov := opt.Provider
	if prov == nil {
		var err error
		prov, err = newProvider(policy.Provider)
		if err != nil {
			return unavailable(req, policy, err.Error()), nil
		}
	}
	if lp, ok := prov.(*LocalProvider); ok {
		lp.Dir = opt.Root
		if !opt.TrustLocal {
			if opt.EnsureTrust == nil {
				return unavailable(req, policy, "repository trust is required for local semantic providers"), nil
			}
			if err := opt.EnsureTrust(); err != nil {
				return unavailable(req, policy, err.Error()), nil
			}
		}
	}

	resp, err := prov.Analyze(ctx, req)
	if err != nil {
		out := unavailable(req, policy, err.Error())
		out.ProviderErr = err
		return out, nil
	}
	findings := resp.Findings
	if findings == nil {
		findings = []Finding{}
	}
	return RunResult{
		Request:  req,
		Findings: findings,
		SemanticRun: &protocol.SemanticRun{
			Enabled:      true,
			Provider:     policy.Provider.Type,
			Enforcement:  policy.EnforcementOrDefault(),
			FindingCount: len(findings),
		},
	}, nil
}

func unavailable(req Request, policy contract.SemanticPolicy, reason string) RunResult {
	return RunResult{
		Request:     req,
		ProviderErr: fmt.Errorf("%s", reason),
		SemanticRun: &protocol.SemanticRun{
			Enabled:     policy.Enabled,
			Provider:    providerType(policy),
			Enforcement: policy.EnforcementOrDefault(),
			Skipped:     reason,
		},
		Findings: nil,
	}
}

func providerType(policy contract.SemanticPolicy) string {
	if policy.Provider == nil {
		return "none"
	}
	return policy.Provider.Type
}

func semanticModesFromSelection(sel impact.Selection) map[string]string {
	m := make(map[string]string, len(sel.Requirements))
	for _, sr := range sel.Requirements {
		mode := sr.Requirement.Verification.Semantic
		if mode == "" {
			mode = "optional"
		}
		m[sr.Requirement.ID] = mode
	}
	return m
}

// ModesFromSelection exposes semantic modes for evidence merge.
func ModesFromSelection(sel impact.Selection) map[string]string {
	return semanticModesFromSelection(sel)
}

package provider

import (
	"context"

	"github.com/hypertrial/intentci/internal/ir"
)

// ManualProvider marks an obligation as requiring human review.
type ManualProvider struct{}

func (p *ManualProvider) Name() string    { return "manual" }
func (p *ManualProvider) Version() string { return "1.0.0" }

func (p *ManualProvider) Validate(spec ir.ProviderSpec) []Diagnostic { return nil }

func (p *ManualProvider) Execute(ctx context.Context, req Request) Result {
	_ = ctx
	return Result{
		Provider: p.Name(), ProviderVersion: p.Version(), Status: "completed",
		DurationMS: 0,
		Evidence: []Evidence{{
			ID: firstNonEmpty(req.Spec.ID, "manual"), Class: "manual",
			Summary: "manual review required", Passed: nil,
			Data: map[string]any{"review_required": true},
		}},
	}
}

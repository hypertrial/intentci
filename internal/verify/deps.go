package verify

import (
	"context"

	"github.com/hypertrial/intentci/internal/attest"
	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/contractdiff"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/internal/scheduler"
	"github.com/hypertrial/intentci/internal/semantic"
)

var openCache = cache.Open

var scheduleChecks = func(ctx context.Context, checks map[string]contract.Check, ids []string, opt scheduler.Options) map[string]runner.Result {
	return scheduler.Run(ctx, checks, ids, opt)
}

var loadBaseContract = contractdiff.LoadBase
var writeAttestation = attest.Write
var buildAttestation = attest.Build

var runSemantic = func(ctx context.Context, opt semantic.RunOptions) (semantic.RunResult, error) {
	return semantic.Run(ctx, opt)
}

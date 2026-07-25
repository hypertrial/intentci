package verify

import (
	"context"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/internal/scheduler"
)

var openCache = cache.Open

var scheduleChecks = func(ctx context.Context, checks map[string]contract.Check, ids []string, opt scheduler.Options) map[string]runner.Result {
	return scheduler.Run(ctx, checks, ids, opt)
}

package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

// Options configures execution.
type Options struct {
	Root         string
	Config       *config.Config
	Registry     *provider.Registry
	BaseCommit   string
	HeadCommit   string
	ChangedFiles []string
	RunID        string
	IRHash       string
	NoCache      bool
	CacheDir     string
}

// LeafResult maps provider leaf id -> result.
type LeafResult map[string]provider.Result

// Run executes all provider leaves for selected requirements.
func Run(ctx context.Context, reqs []ir.Requirement, opt Options) (map[string]LeafResult, []verdict.RequirementResult) {
	// Successful-result caching remains disabled until the key includes complete
	// repository, provider, input, and environment provenance.
	opt.NoCache = true
	if opt.Registry == nil {
		opt.Registry = provider.DefaultRegistry()
	}
	timeout, _ := config.ParseDuration(opt.Config.Verification.DefaultTimeout)
	max := opt.Config.MaxParallelOr(4)

	type job struct {
		reqID string
		oblID string
		key   string
		spec  ir.ProviderSpec
	}
	type prepared struct {
		req    ir.Requirement
		obl    ir.Obligation
		verify ir.VerifyNode
		jobs   []job
	}
	var preparedObs []prepared
	var jobs []job
	for _, r := range reqs {
		for _, o := range r.Obligations {
			counter := 0
			verify := assignLeafIDs(o.Verify, &counter)
			var oblJobs []job
			collectLeaves(verify, func(spec ir.ProviderSpec) {
				key := spec.ID
				j := job{reqID: r.ID, oblID: o.ID, key: key, spec: spec}
				oblJobs = append(oblJobs, j)
				jobs = append(jobs, j)
			})
			preparedObs = append(preparedObs, prepared{req: r, obl: o, verify: verify, jobs: oblJobs})
		}
	}

	results := make(map[string]provider.Result)
	var mu sync.Mutex
	sem := make(chan struct{}, max)
	var wg sync.WaitGroup

	for _, j := range jobs {
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			p, ok := opt.Registry.Get(j.spec.Provider)
			var res provider.Result
			if !ok {
				res = provider.Result{
					Provider: j.spec.Provider, Status: "error",
					Diagnostics: []string{"unknown provider"},
					Evidence: []provider.Evidence{{
						ID: j.key, Class: "deterministic", Summary: "unknown provider", Passed: boolPtr(false),
					}},
				}
			} else {
				res = p.Execute(ctx, provider.Request{
					RunID: opt.RunID, RequirementID: j.reqID, ObligationID: j.oblID,
					Root: opt.Root, BaseCommit: opt.BaseCommit, HeadCommit: opt.HeadCommit,
					ChangedFiles: opt.ChangedFiles, Spec: j.spec, Timeout: timeout,
					RetainStdout: opt.Config.Evidence.RetainStdout,
					RetainStderr: opt.Config.Evidence.RetainStderr,
				})
			}
			mu.Lock()
			results[j.reqID+"/"+j.oblID+"/"+j.key] = res
			mu.Unlock()
		}()
	}
	wg.Wait()

	if mutateResults != nil {
		mutateResults(results)
	}

	byReq := map[string]LeafResult{}
	reqObs := map[string][]verdict.ObligationResult{}
	for _, prep := range preparedObs {
		leaves := LeafResult{}
		for _, j := range prep.jobs {
			if res, ok := results[j.reqID+"/"+j.oblID+"/"+j.key]; ok {
				leaves[j.key] = res
			}
		}
		if byReq[prep.req.ID] == nil {
			byReq[prep.req.ID] = LeafResult{}
		}
		for k, res := range leaves {
			byReq[prep.req.ID][k] = res
			byReq[prep.req.ID][prep.obl.ID+"/"+k] = res
		}
		v, reason, ev := verdict.EvaluateNode(prep.verify, leaves)
		reqObs[prep.req.ID] = append(reqObs[prep.req.ID], verdict.ObligationResult{
			ID: prep.obl.ID, Statement: prep.obl.Statement, Required: prep.obl.Required,
			Verdict: v, Reason: reason, Evidence: ev,
		})
	}

	var reqResults []verdict.RequirementResult
	for _, r := range reqs {
		obs := reqObs[r.ID]
		if obs == nil {
			obs = []verdict.ObligationResult{}
		}
		reqResults = append(reqResults, verdict.AggregateRequirement(r, obs))
	}
	return byReq, reqResults
}

// assignLeafIDs returns a copy of n with unique provider leaf IDs filled in.
func assignLeafIDs(n ir.VerifyNode, counter *int) ir.VerifyNode {
	out := n
	if n.Provider != nil {
		p := *n.Provider
		if p.ID == "" {
			*counter++
			p.ID = fmt.Sprintf("%s#%d", p.Provider, *counter)
		}
		out.Provider = &p
	}
	if len(n.All) > 0 {
		out.All = make([]ir.VerifyNode, len(n.All))
		for i, c := range n.All {
			out.All[i] = assignLeafIDs(c, counter)
		}
	}
	if len(n.Any) > 0 {
		out.Any = make([]ir.VerifyNode, len(n.Any))
		for i, c := range n.Any {
			out.Any[i] = assignLeafIDs(c, counter)
		}
	}
	if n.Not != nil {
		child := assignLeafIDs(*n.Not, counter)
		out.Not = &child
	}
	return out
}

func collectLeaves(n ir.VerifyNode, fn func(ir.ProviderSpec)) {
	if n.Provider != nil {
		fn(*n.Provider)
	}
	for _, c := range n.All {
		collectLeaves(c, fn)
	}
	for _, c := range n.Any {
		collectLeaves(c, fn)
	}
	if n.Not != nil {
		collectLeaves(*n.Not, fn)
	}
}

func split3(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			out = append(out, s[start:i])
			start = i + 1
			if len(out) == 2 {
				out = append(out, s[start:])
				return out
			}
		}
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }

// mutateResults is an optional test hook applied after provider execution.
var mutateResults func(map[string]provider.Result)

// Ensure timeout import used
var _ = time.Second

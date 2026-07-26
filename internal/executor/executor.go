package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
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
	var jobs []job
	for _, r := range reqs {
		for _, o := range r.Obligations {
			collectLeaves(o.Verify, func(spec ir.ProviderSpec) {
				key := spec.ID
				if key == "" {
					key = spec.Provider
				}
				jobs = append(jobs, job{reqID: r.ID, oblID: o.ID, key: key, spec: spec})
			})
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

			cacheKey := cacheKey(opt.IRHash, j.spec, opt.ChangedFiles)
			if !opt.NoCache {
				if cached, ok := loadCache(opt.CacheDir, cacheKey); ok {
					cached.FromCache = true
					mu.Lock()
					results[j.reqID+"/"+j.oblID+"/"+j.key] = cached
					mu.Unlock()
					return
				}
			}

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
			if !opt.NoCache && res.Status == "completed" && allPassed(res) {
				_ = saveCache(opt.CacheDir, cacheKey, res)
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
	for k, res := range results {
		// k = req/obl/key
		parts := split3(k)
		if len(parts) != 3 {
			continue
		}
		lr := byReq[parts[0]]
		if lr == nil {
			lr = LeafResult{}
			byReq[parts[0]] = lr
		}
		// also index by obl-specific and plain key
		lr[parts[2]] = res
		lr[parts[1]+"/"+parts[2]] = res
	}

	var reqResults []verdict.RequirementResult
	for _, r := range reqs {
		leaves := byReq[r.ID]
		if leaves == nil {
			leaves = LeafResult{}
		}
		var obs []verdict.ObligationResult
		for _, o := range r.Obligations {
			v, reason, ev := verdict.EvaluateNode(o.Verify, leaves)
			obs = append(obs, verdict.ObligationResult{
				ID: o.ID, Statement: o.Statement, Required: o.Required,
				Verdict: v, Reason: reason, Evidence: ev,
			})
		}
		reqResults = append(reqResults, verdict.AggregateRequirement(r, obs))
	}
	return byReq, reqResults
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

func cacheKey(irHash string, spec ir.ProviderSpec, changed []string) string {
	b, _ := json.Marshal(struct {
		IR      string
		Spec    ir.ProviderSpec
		Changed []string
	}{irHash, spec, changed})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func loadCache(dir, key string) (provider.Result, bool) {
	if dir == "" {
		return provider.Result{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		return provider.Result{}, false
	}
	var res provider.Result
	if err := json.Unmarshal(data, &res); err != nil {
		return provider.Result{}, false
	}
	return res, true
}

func saveCache(dir, key string, res provider.Result) error {
	if dir == "" {
		return nil
	}
	if err := mkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, key+".json"), raw, 0o644)
}

func allPassed(res provider.Result) bool {
	if res.Status != "completed" {
		return false
	}
	for _, e := range res.Evidence {
		if e.Passed == nil || !*e.Passed {
			return false
		}
	}
	return len(res.Evidence) > 0
}

func boolPtr(b bool) *bool { return &b }

// mutateResults is an optional test hook applied after provider execution.
var mutateResults func(map[string]provider.Result)

var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile

// Ensure timeout import used
var _ = time.Second

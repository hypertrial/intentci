package scheduler

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Options configures the scheduler.
type Options struct {
	Dir          string
	MaxParallel  int
	Stdout       io.Writer
	Stderr       io.Writer
	Cache        *cache.Store
	NoCache      bool
	ContractHash string
	ChangeHash   string
	EnvInclude   []string
}

// Run executes checks respecting depends_on, exclusive, concurrency, and cache.
func Run(ctx context.Context, checks map[string]contract.Check, ids []string, opt Options) map[string]runner.Result {
	results := make(map[string]runner.Result)
	var mu sync.Mutex

	pending := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := checks[id]; ok {
			pending[id] = struct{}{}
		}
	}

	max := opt.MaxParallel
	if max <= 0 {
		max = numCPU()
		if max < 1 {
			max = 1
		}
	}

	var streamMu sync.Mutex
	stdout := lockedWriter{w: opt.Stdout, mu: &streamMu}
	stderr := lockedWriter{w: opt.Stderr, mu: &streamMu}

	for len(pending) > 0 {
		ready := readyChecks(pending, checks, results)
		if len(ready) == 0 {
			for id := range pending {
				results[id] = runner.Result{
					Check:  checks[id],
					Status: protocol.CheckSkipped,
					Reason: "skipped because a dependency did not pass",
				}
				delete(pending, id)
			}
			break
		}

		exclusive := filterExclusive(ready, checks)
		batch := ready
		if len(exclusive) > 0 {
			batch = exclusive[:1]
		} else if len(batch) > max {
			batch = batch[:max]
		}

		var wg sync.WaitGroup
		for _, id := range batch {
			id := id
			delete(pending, id)
			wg.Add(1)
			go func() {
				defer wg.Done()
				ch := checks[id]
				if !opt.NoCache && opt.Cache != nil {
					key, ok, err := cache.Key(cache.KeyInput{
						Check:        ch,
						ContractHash: opt.ContractHash,
						ChangeHash:   opt.ChangeHash,
						RepoRoot:     opt.Dir,
						EnvInclude:   opt.EnvInclude,
					})
					if err == nil && ok {
						if cached, hit := opt.Cache.Get(key); hit {
							cached.Check = ch
							cached.FromCache = true
							cached.Reason = "restored from cache"
							mu.Lock()
							results[id] = cached
							mu.Unlock()
							return
						}
					}
				}

				outW := prefixWriter(stdout.writer(), id)
				errW := prefixWriter(stderr.writer(), id)
				res := runner.Run(ctx, ch, runner.Options{
					Dir:    opt.Dir,
					Stdout: outW,
					Stderr: errW,
				})
				flushPrefix(outW)
				flushPrefix(errW)
				if !opt.NoCache && opt.Cache != nil && res.Status == protocol.CheckPass {
					key, ok, err := cache.Key(cache.KeyInput{
						Check:        ch,
						ContractHash: opt.ContractHash,
						ChangeHash:   opt.ChangeHash,
						RepoRoot:     opt.Dir,
						EnvInclude:   opt.EnvInclude,
					})
					if err == nil && ok {
						_ = opt.Cache.Put(key, res)
					}
				}
				mu.Lock()
				results[id] = res
				mu.Unlock()
			}()
		}
		wg.Wait()
	}

	return results
}

func readyChecks(pending map[string]struct{}, checks map[string]contract.Check, results map[string]runner.Result) []string {
	var ready []string
	for id := range pending {
		ch := checks[id]
		ok := true
		for _, dep := range ch.DependsOn {
			res, done := results[dep]
			if !done {
				ok = false
				break
			}
			if res.Status != protocol.CheckPass {
				ok = false
				break
			}
		}
		if ok {
			ready = append(ready, id)
		}
	}
	return ready
}

func filterExclusive(ids []string, checks map[string]contract.Check) []string {
	var out []string
	for _, id := range ids {
		if checks[id].Exclusive {
			out = append(out, id)
		}
	}
	return out
}

type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (l lockedWriter) writer() io.Writer {
	if l.w == nil {
		return nil
	}
	return l
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

type prefWriter struct {
	w   io.Writer
	id  string
	buf []byte
}

func prefixWriter(w io.Writer, id string) io.Writer {
	if w == nil {
		return nil
	}
	return &prefWriter{w: w, id: id}
}

func (p *prefWriter) Write(b []byte) (int, error) {
	p.buf = append(p.buf, b...)
	for {
		i := indexByte(p.buf, '\n')
		if i < 0 {
			break
		}
		line := p.buf[:i+1]
		p.buf = p.buf[i+1:]
		if _, err := fmt.Fprintf(p.w, "[%s] %s", p.id, line); err != nil {
			return len(b), err
		}
	}
	return len(b), nil
}

func flushPrefix(w io.Writer) {
	p, ok := w.(*prefWriter)
	if !ok || p == nil || len(p.buf) == 0 {
		return
	}
	_, _ = fmt.Fprintf(p.w, "[%s] %s\n", p.id, p.buf)
	p.buf = nil
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

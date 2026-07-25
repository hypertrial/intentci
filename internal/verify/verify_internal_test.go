package verify

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/trust"
)

func TestRun_CacheOpenAndNoChecks(t *testing.T) {
	oldOpen := openCache
	defer func() { openCache = oldOpen }()
	openCache = func(string) (*cache.Store, error) { return nil, errors.New("cache") }

	dir := setupRepo(t)
	var out, errb bytes.Buffer
	if _, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", Trust: true, Stdout: &out, Stderr: &errb,
	}); err == nil {
		t.Fatal("cache open")
	}

	openCache = cache.Open
	body := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: advisory
    applies_to: {include: ["nope/**"]}
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    profiles: [full]
    inputs: ["nope/**"]
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body), 0o644)
	out.Reset()
	o, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", Trust: true, Stdout: &out, Stderr: &errb, Stream: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.Result == nil || len(o.Result.Checks) != 0 {
		t.Fatalf("no checks path: %+v", o.Result)
	}

	cfg := t.TempDir()
	oldCfg := trust.SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer trust.SetUserConfigDir(oldCfg)
	body2 := `version: 1
product: {name: x, purpose: y}
policy: {default_base: HEAD}
requirements:
  - id: BUILD-001
    type: reliability
    title: t
    statement: s
    status: approved
    severity: blocking
    applies_to: {include: ["**"]}
    verification: {checks: [go-test]}
checks:
  - id: go-test
    command: "true"
    profiles: [full]
    inputs: ["**"]
    timeout: 1m
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(body2), 0o644)
	if _, err := Run(context.Background(), Options{
		Root: dir, Base: "HEAD", Profile: "full", Trust: false, Stderr: &errb,
	}); err == nil {
		t.Fatal("trust required")
	}
}

func setupRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "-c", "core.hooksPath=/dev/null", "init")
	runGit(t, dir, "git", "checkout", "-b", "main")
	os.MkdirAll(filepath.Join(dir, ".intentci"), 0o755)
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init", "--allow-empty")
	return dir
}

func runGit(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

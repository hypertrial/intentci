package verify_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/cache"
	"github.com/hypertrial/intentci/internal/trust"
	"github.com/hypertrial/intentci/internal/verify"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "-c", "core.hooksPath=/dev/null", "init")
	runGit(t, dir, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "init")
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

func TestVerify_ChangeAndCache(t *testing.T) {
	dir := gitRepo(t)
	cfg := t.TempDir()
	oldCfg := trust.SetUserConfigDir(func() (string, error) { return cfg, nil })
	defer trust.SetUserConfigDir(oldCfg)
	cacheDir := t.TempDir()
	oldCache := cache.SetUserCacheDir(func() (string, error) { return cacheDir, nil })
	defer cache.SetUserCacheDir(oldCache)

	os.MkdirAll(filepath.Join(dir, ".intentci", "changes"), 0o755)
	contractBody := `version: 1
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
    profiles: [fast, full]
    inputs: ["**"]
    timeout: 1m
    cache: success
`
	os.WriteFile(filepath.Join(dir, ".intentci", "contract.yaml"), []byte(contractBody), 0o644)
	change := `version: 1
id: DEMO-1
status: approved
type: feature
summary: demo
goals: [g]
acceptance:
  - id: AC-001
    statement: works
    severity: blocking
    verification: {checks: [go-test]}
affected_requirements: [BUILD-001]
required_checks: [go-test]
`
	os.WriteFile(filepath.Join(dir, ".intentci", "changes", "DEMO-1.yaml"), []byte(change), 0o644)

	var out, errb bytes.Buffer
	o, err := verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", Trust: true, ChangeID: "DEMO-1",
		Stdout: &out, Stderr: &errb, Stream: true,
	})
	if err != nil || o.ExitCode != 0 {
		t.Fatalf("%v %+v %s", err, o, errb.String())
	}
	if o.Result.ChangeSpec == nil || o.Result.Status != protocol.StatusPass {
		t.Fatalf("%+v", o.Result)
	}
	o2, err := verify.Run(context.Background(), verify.Options{
		Root: dir, Base: "HEAD", Profile: "full", Trust: true, ChangeID: "DEMO-1",
		Stdout: &out, Stderr: &errb,
	})
	if err != nil {
		t.Fatal(err)
	}
	if o2.Result.Summary.ChecksCached < 1 {
		t.Fatalf("expected cache %+v", o2.Result.Summary)
	}
	_, err = verify.Run(context.Background(), verify.Options{Root: dir, ChangeID: "NOPE", Trust: true, Base: "HEAD"})
	if err == nil {
		t.Fatal("missing change")
	}
	_, err = verify.Run(context.Background(), verify.Options{Root: t.TempDir(), Trust: true})
	if err == nil {
		t.Fatal("missing contract")
	}
	if _, err := os.Stat(verify.LastResultPath(dir)); err != nil {
		t.Fatal(err)
	}
}

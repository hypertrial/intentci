package app

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hypertrial/intentci/v2/internal/config"
)

func command(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	command(t, root, "git", "init", "-q")
	command(t, root, "git", "config", "user.email", "intentci@example.com")
	command(t, root, "git", "config", "user.name", "IntentCI")
	return root
}

func commitAll(t *testing.T, root string) {
	t.Helper()
	command(t, root, "git", "add", ".")
	command(t, root, "git", "commit", "-qm", "state")
}

func runFrom(t *testing.T, ctx context.Context, root string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := RunFrom(ctx, root, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestSelectChecks(t *testing.T) {
	checks := []config.Check{
		{ID: "go", Paths: []string{"**/*.go"}},
		{ID: "docs", Paths: []string{"docs/**"}},
	}
	selected := selectChecks(checks, []string{"README.md", "x.go", "x.go"})
	if len(selected) != 1 || selected[0].ID != "go" {
		t.Fatalf("selected = %#v", selected)
	}
	if got := selectChecks(checks, []string{"README.md"}); got != nil {
		t.Fatalf("selected = %#v, want nil", got)
	}
	if got := selectChecks(checks, []string{config.FileName}); !reflect.DeepEqual(got, checks) {
		t.Fatalf("config selection = %#v", got)
	}
}

func TestDetectStacksAndPriority(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		id    string
		run   string
	}{
		{"go", []string{"go.mod"}, "go-tests", "go test ./..."},
		{"pnpm", []string{"package.json", "pnpm-lock.yaml"}, "node-tests", "pnpm test"},
		{"yarn", []string{"package.json", "yarn.lock"}, "node-tests", "yarn test"},
		{"npm", []string{"package.json"}, "node-tests", "npm test"},
		{"uv", []string{"pyproject.toml", "uv.lock"}, "python-tests", "uv run pytest -q"},
		{"python", []string{"pyproject.toml"}, "python-tests", "python3 -m pytest -q"},
		{"rust", []string{"Cargo.toml"}, "rust-tests", "cargo test"},
		{"maven wrapper", []string{"pom.xml", "mvnw"}, "java-tests", "./mvnw test"},
		{"maven", []string{"pom.xml"}, "java-tests", "mvn test"},
		{"gradle wrapper", []string{"build.gradle", "gradlew"}, "java-tests", "./gradlew test"},
		{"gradle", []string{"build.gradle.kts"}, "java-tests", "gradle test"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, file := range test.files {
				writeFile(t, root, file, "")
			}
			got := detect(root)
			if got.ID != test.id || got.Run != test.run {
				t.Fatalf("detect() = %s/%q, want %s/%q", got.ID, got.Run, test.id, test.run)
			}
		})
	}
	root := t.TempDir()
	writeFile(t, root, "package.json", "{}")
	writeFile(t, root, "go.mod", "module example")
	if got := detect(root); got.ID != "go-tests" {
		t.Fatalf("priority selected %q", got.ID)
	}
}

func TestInitialize(t *testing.T) {
	root := gitRepo(t)
	if err := initialize(root); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Checks[0].ID != "tests" || !strings.Contains(cfg.Checks[0].Run, "exit 1") {
		t.Fatalf("placeholder = %#v", cfg.Checks[0])
	}
	if err := initialize(root); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second initialize error = %v", err)
	}

	v1 := gitRepo(t)
	writeFile(t, v1, ".intentci/config.yaml", "version: 1")
	if err := initialize(v1); err == nil || !strings.Contains(err.Error(), "v1 configuration") {
		t.Fatalf("v1 initialize error = %v", err)
	}
}

func TestCLIHelpVersionAndUsage(t *testing.T) {
	code, out, _ := runFrom(t, context.Background(), t.TempDir(), "--help")
	if code != 0 || !strings.Contains(out, "intentci --all") {
		t.Fatalf("help = %d, %q", code, out)
	}
	code, out, _ = runFrom(t, context.Background(), t.TempDir(), "version")
	if code != 0 || strings.TrimSpace(out) != "2.0.1" {
		t.Fatalf("version = %d, %q", code, out)
	}
	root := gitRepo(t)
	code, _, stderr := runFrom(t, context.Background(), root, "verify")
	if code != 2 || !strings.Contains(stderr, "unsupported arguments") {
		t.Fatalf("usage = %d, %q", code, stderr)
	}
	code, _, stderr = runFrom(t, context.Background(), t.TempDir())
	if code != 2 || !strings.Contains(stderr, "not a Git repository") {
		t.Fatalf("non-repo = %d, %q", code, stderr)
	}
}

func TestInitAndChangedFileWorkflow(t *testing.T) {
	root := gitRepo(t)
	writeFile(t, root, "go.mod", "module example\n\ngo 1.23\n")
	code, stdout, stderr := runFrom(t, context.Background(), root, "init")
	if code != 0 {
		t.Fatalf("init = %d\n%s\n%s", code, stdout, stderr)
	}
	writeFile(t, root, "main.go", "package example\n")
	commitAll(t, root)

	code, stdout, stderr = runFrom(t, context.Background(), filepath.Join(root, "."))
	if code != 0 || !strings.Contains(stdout, "No changes") {
		t.Fatalf("clean = %d\n%s\n%s", code, stdout, stderr)
	}
	code, stdout, stderr = runFrom(t, context.Background(), root, "--all")
	if code != 0 || !strings.Contains(stdout, "RUN go-tests") {
		t.Fatalf("clean all = %d\n%s\n%s", code, stdout, stderr)
	}
	writeFile(t, root, "README.md", "unrelated")
	code, stdout, stderr = runFrom(t, context.Background(), root)
	if code != 0 || !strings.Contains(stdout, "No checks match") {
		t.Fatalf("unrelated = %d\n%s\n%s", code, stdout, stderr)
	}
	writeFile(t, root, "main.go", "package example\n\nconst Changed = true\n")
	subdir := filepath.Join(root, "nested")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runFrom(t, context.Background(), subdir)
	if code != 0 || !strings.Contains(stdout, "RUN go-tests") || !strings.Contains(stdout, "PASS 1 check") {
		t.Fatalf("changed = %d\n%s\n%s", code, stdout, stderr)
	}
}

func TestMatchingEnvironmentRootAndAll(t *testing.T) {
	root := gitRepo(t)
	toolDir := t.TempDir()
	writeFile(t, toolDir, "intentci-path-probe", "#!/bin/sh\nexit 0\n")
	profileDir := t.TempDir()
	writeFile(t, profileDir, ".zprofile", "export PATH=/usr/bin:/bin\n")
	writeFile(t, root, config.FileName, `version: 2
checks:
  - id: source
    intent: Source check.
    paths: ["src/**"]
    run: intentci-path-probe && test "$INTENTCI_TEST_VALUE" = inherited && pwd > where.txt
  - id: docs
    intent: Docs check.
    paths: ["docs/**"]
    run: echo docs
`)
	writeFile(t, root, "src/file.txt", "initial")
	commitAll(t, root)
	writeFile(t, root, "src/file.txt", "changed")
	t.Setenv("INTENTCI_TEST_VALUE", "inherited")
	t.Setenv("PATH", toolDir+":"+os.Getenv("PATH"))
	t.Setenv("ZDOTDIR", profileDir)

	code, stdout, stderr := runFrom(t, context.Background(), root)
	if code != 0 || strings.Contains(stdout, "RUN docs") {
		t.Fatalf("matching = %d\n%s\n%s", code, stdout, stderr)
	}
	where, err := os.ReadFile(filepath.Join(root, "where.txt"))
	if err != nil || strings.TrimSpace(string(where)) != root {
		t.Fatalf("working directory = %q, %v; want %q", where, err, root)
	}
	code, stdout, stderr = runFrom(t, context.Background(), root, "--all")
	if code != 0 || !strings.Contains(stdout, "RUN source") || !strings.Contains(stdout, "RUN docs") {
		t.Fatalf("all = %d\n%s\n%s", code, stdout, stderr)
	}
}

func TestFailFastInvalidConfigAndLaunchFailure(t *testing.T) {
	root := gitRepo(t)
	writeFile(t, root, config.FileName, `version: 2
checks:
  - id: fail
    intent: This fails.
    paths: ["**"]
    run: exit 7
  - id: later
    intent: This must not run.
    paths: ["**"]
    run: touch later
`)
	code, _, stderr := runFrom(t, context.Background(), root, "--all")
	if code != 1 || !strings.Contains(stderr, "FAIL fail (exit 7)") {
		t.Fatalf("failure = %d, %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "later")); !os.IsNotExist(err) {
		t.Fatalf("later check ran: %v", err)
	}

	writeFile(t, root, config.FileName, "version: 2\nchecks: []\n")
	code, _, stderr = runFrom(t, context.Background(), root, "--all")
	if code != 2 || !strings.Contains(stderr, "checks must not be empty") {
		t.Fatalf("invalid config = %d, %q", code, stderr)
	}

	writeFile(t, root, config.FileName, `version: 2
checks:
  - id: launch
    intent: Shell starts.
    paths: ["**"]
    run: "true"
`)
	oldShell := shellPath
	shellPath = filepath.Join(root, "missing-zsh")
	defer func() { shellPath = oldShell }()
	code, _, stderr = runFrom(t, context.Background(), root, "--all")
	if code != 2 || !strings.Contains(stderr, "start launch") {
		t.Fatalf("launch = %d, %q", code, stderr)
	}
}

func TestCancellationAndSignalExitCodes(t *testing.T) {
	root := gitRepo(t)
	writeFile(t, root, config.FileName, `version: 2
checks:
  - id: wait
    intent: Waits.
    paths: ["**"]
    run: sleep 0.3; touch child-survived
  - id: later
    intent: Must not run.
    paths: ["**"]
    run: touch later
`)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	code, _, stderr := runFrom(t, ctx, root, "--all")
	if code != 130 || !strings.Contains(stderr, "INTERRUPTED wait") {
		t.Fatalf("cancel = %d, %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "later")); !os.IsNotExist(err) {
		t.Fatalf("later check ran: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(root, "child-survived")); !os.IsNotExist(err) {
		t.Fatalf("child process survived cancellation: %v", err)
	}
	if exitCodeForSignal(os.Interrupt) != 130 || exitCodeForSignal(syscall.SIGTERM) != 143 {
		t.Fatal("signal exit mapping changed")
	}
}

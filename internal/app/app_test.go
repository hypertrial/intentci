package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hypertrial/intentci/v2/internal/config"
	"github.com/hypertrial/intentci/v2/internal/version"
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

func detectionCases() []struct {
	name  string
	files []string
	want  config.Check
} {
	return []struct {
		name  string
		files []string
		want  config.Check
	}{
		{"go", []string{"go.mod"}, config.Check{
			ID: "go-tests", Intent: "Go changes must keep tests passing.",
			Paths: []string{"**/*.go", "go.mod", "go.sum"}, Run: "go test ./...",
		}},
		{"pnpm", []string{"package.json", "pnpm-lock.yaml"}, config.Check{
			ID: "node-tests", Intent: "Node changes must keep tests passing.",
			Paths: []string{
				"**/*.js", "**/*.mjs", "**/*.cjs", "**/*.ts", "**/*.tsx",
				"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
			},
			Run: "pnpm test",
		}},
		{"yarn", []string{"package.json", "yarn.lock"}, config.Check{
			ID: "node-tests", Intent: "Node changes must keep tests passing.",
			Paths: []string{
				"**/*.js", "**/*.mjs", "**/*.cjs", "**/*.ts", "**/*.tsx",
				"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
			},
			Run: "yarn test",
		}},
		{"npm", []string{"package.json"}, config.Check{
			ID: "node-tests", Intent: "Node changes must keep tests passing.",
			Paths: []string{
				"**/*.js", "**/*.mjs", "**/*.cjs", "**/*.ts", "**/*.tsx",
				"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
			},
			Run: "npm test",
		}},
		{"uv", []string{"pyproject.toml", "uv.lock"}, config.Check{
			ID: "python-tests", Intent: "Python changes must keep tests passing.",
			Paths: []string{"**/*.py", "pyproject.toml", "requirements*.txt", "uv.lock"},
			Run:   "uv run pytest -q",
		}},
		{"python", []string{"pyproject.toml"}, config.Check{
			ID: "python-tests", Intent: "Python changes must keep tests passing.",
			Paths: []string{"**/*.py", "pyproject.toml", "requirements*.txt", "uv.lock"},
			Run:   "python3 -m pytest -q",
		}},
		{"rust", []string{"Cargo.toml"}, config.Check{
			ID: "rust-tests", Intent: "Rust changes must keep tests passing.",
			Paths: []string{"**/*.rs", "Cargo.toml", "Cargo.lock"}, Run: "cargo test",
		}},
		{"maven wrapper", []string{"pom.xml", "mvnw"}, config.Check{
			ID: "java-tests", Intent: "Java changes must keep tests passing.",
			Paths: []string{"**/*.java", "pom.xml", ".mvn/**", "mvnw", "mvnw.cmd"},
			Run:   "./mvnw test",
		}},
		{"maven", []string{"pom.xml"}, config.Check{
			ID: "java-tests", Intent: "Java changes must keep tests passing.",
			Paths: []string{"**/*.java", "pom.xml", ".mvn/**", "mvnw", "mvnw.cmd"},
			Run:   "mvn test",
		}},
		{"gradle wrapper", []string{"build.gradle", "gradlew"}, config.Check{
			ID: "java-tests", Intent: "Java changes must keep tests passing.",
			Paths: []string{
				"**/*.java", "build.gradle", "build.gradle.kts",
				"settings.gradle", "settings.gradle.kts", "gradle/**", "gradle.lockfile",
				"gradlew", "gradlew.bat",
			},
			Run: "./gradlew test",
		}},
		{"gradle", []string{"build.gradle.kts"}, config.Check{
			ID: "java-tests", Intent: "Java changes must keep tests passing.",
			Paths: []string{
				"**/*.java", "build.gradle", "build.gradle.kts",
				"settings.gradle", "settings.gradle.kts", "gradle/**", "gradle.lockfile",
				"gradlew", "gradlew.bat",
			},
			Run: "gradle test",
		}},
		{"gradle settings", []string{"settings.gradle"}, config.Check{
			ID: "java-tests", Intent: "Java changes must keep tests passing.",
			Paths: []string{
				"**/*.java", "build.gradle", "build.gradle.kts",
				"settings.gradle", "settings.gradle.kts", "gradle/**", "gradle.lockfile",
				"gradlew", "gradlew.bat",
			},
			Run: "gradle test",
		}},
		{"unknown", nil, config.Check{
			ID: "tests", Intent: "Repository changes must pass its configured tests.",
			Paths: []string{"**"},
			Run:   `echo "Edit .intentci.yaml and replace this command." >&2; exit 1`,
		}},
	}
}

func TestDetectStacks(t *testing.T) {
	for _, test := range detectionCases() {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, file := range test.files {
				writeFile(t, root, file, "")
			}
			got := detect(root)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("detect() = %#v\nwant %#v", got, test.want)
			}
		})
	}
}

func TestDetectPriority(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		run   string
	}{
		{"go before all", []string{"go.mod", "package.json", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle"}, "go test ./..."},
		{"node before later stacks", []string{"package.json", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle"}, "npm test"},
		{"python before later stacks", []string{"pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle"}, "python3 -m pytest -q"},
		{"rust before java", []string{"Cargo.toml", "pom.xml", "build.gradle"}, "cargo test"},
		{"maven before gradle", []string{"pom.xml", "build.gradle"}, "mvn test"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, file := range test.files {
				writeFile(t, root, file, "")
			}
			if got := detect(root); got.Run != test.run {
				t.Fatalf("priority selected %q, want %q", got.Run, test.run)
			}
		})
	}
}

func TestInitializeStacks(t *testing.T) {
	for _, test := range detectionCases() {
		t.Run(test.name, func(t *testing.T) {
			root := gitRepo(t)
			for _, file := range test.files {
				writeFile(t, root, file, "")
			}
			if err := initialize(root); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Checks) != 1 || !reflect.DeepEqual(cfg.Checks[0], test.want) {
				t.Fatalf("initialized checks = %#v\nwant %#v", cfg.Checks, test.want)
			}
		})
	}
}

func TestInitializeRefusesOverwriteAndV1(t *testing.T) {
	root := gitRepo(t)
	if err := initialize(root); err != nil {
		t.Fatal(err)
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
	if code != 0 || strings.TrimSpace(out) != version.DefaultVersion {
		t.Fatalf("version = %d, %q", code, out)
	}
	root := gitRepo(t)
	code, _, stderr := runFrom(t, context.Background(), root, "verify")
	if code != 2 || !strings.Contains(stderr, "unsupported arguments") {
		t.Fatalf("usage = %d, %q", code, stderr)
	}
	code, _, stderr = runFrom(t, context.Background(), root, "-h")
	if code != 2 || !strings.Contains(stderr, "unsupported arguments") {
		t.Fatalf("short help usage = %d, %q", code, stderr)
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

func TestRepositoryCancellationOutput(t *testing.T) {
	fakeBin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "started")
	writeFile(t, fakeBin, "git", `#!/bin/zsh
print started > "$INTENTCI_GIT_MARKER"
sleep 30
`)
	t.Setenv("PATH", fakeBin+":"+os.Getenv("PATH"))
	t.Setenv("INTENTCI_GIT_MARKER", marker)

	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		code   int
		stderr string
	}
	done := make(chan result, 1)
	start := t.TempDir()
	go func() {
		code, _, stderr := runFrom(t, ctx, start)
		done <- result{code: code, stderr: stderr}
	}()
	waitForPath(t, marker)
	cancel()
	select {
	case got := <-done:
		if got.code != 130 || !strings.Contains(got.stderr, "intentci: interrupted") ||
			strings.Contains(got.stderr, "not a Git repository") {
			t.Fatalf("canceled repository discovery = %d, %q", got.code, got.stderr)
		}
	case <-time.After(time.Second):
		t.Fatal("repository discovery ignored cancellation")
	}
}

func TestMainSIGINTStopsStubbornGroup(t *testing.T) {
	root := gitRepo(t)
	writeFile(t, root, "child.zsh", `#!/bin/zsh
trap '' TERM
echo $$ > child.pid
sleep 30
`)
	writeFile(t, root, "outer.zsh", `#!/bin/zsh
trap '' TERM
./child.zsh &
echo $$ > leader.pid
wait
`)
	writeFile(t, root, config.FileName, `version: 2
checks:
  - id: stubborn
    intent: Stubborn processes must stop.
    paths: ["**"]
    run: ./outer.zsh
  - id: later
    intent: Later checks must not run.
    paths: ["**"]
    run: touch later
`)

	command, _, stderr := mainHelper(root)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	leader := waitPIDFile(t, filepath.Join(root, "leader.pid"))
	child := waitPIDFile(t, filepath.Join(root, "child.pid"))
	defer syscall.Kill(-leader, syscall.SIGKILL)
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if code := waitExitCode(t, command); code != 130 {
		t.Fatalf("SIGINT exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "INTERRUPTED stubborn") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	waitProcessGone(t, leader)
	waitProcessGone(t, child)
	if _, err := os.Stat(filepath.Join(root, "later")); !os.IsNotExist(err) {
		t.Fatalf("later check ran: %v", err)
	}
}

func TestMainSIGTERMExitCode(t *testing.T) {
	root := gitRepo(t)
	writeFile(t, root, "wait.zsh", `#!/bin/zsh
echo $$ > leader.pid
sleep 30 &
echo $! > child.pid
wait
`)
	writeFile(t, root, config.FileName, `version: 2
checks:
  - id: wait
    intent: Interruption must stop this check.
    paths: ["**"]
    run: ./wait.zsh
  - id: later
    intent: Later checks must not run.
    paths: ["**"]
    run: touch later
`)

	command, _, stderr := mainHelper(root)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	leader := waitPIDFile(t, filepath.Join(root, "leader.pid"))
	child := waitPIDFile(t, filepath.Join(root, "child.pid"))
	defer syscall.Kill(-leader, syscall.SIGKILL)
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := waitExitCode(t, command); code != 143 {
		t.Fatalf("SIGTERM exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "INTERRUPTED wait\n") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	waitProcessGone(t, leader)
	waitProcessGone(t, child)
	if _, err := os.Stat(filepath.Join(root, "later")); !os.IsNotExist(err) {
		t.Fatalf("later check ran: %v", err)
	}
}

func TestMainSignalHelper(t *testing.T) {
	if os.Getenv("INTENTCI_MAIN_HELPER") != "1" {
		return
	}
	os.Exit(Main([]string{"--all"}, os.Stdout, os.Stderr))
}

func mainHelper(root string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	command := exec.Command(os.Args[0], "-test.run=^TestMainSignalHelper$")
	command.Dir = root
	command.Env = append(os.Environ(), "INTENTCI_MAIN_HELPER=1")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	return command, &stdout, &stderr
}

func waitExitCode(t *testing.T, command *exec.Cmd) int {
	t.Helper()
	err := command.Wait()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("command.Wait() = %v", err)
	}
	return exitError.ExitCode()
}

func waitForPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func waitPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for PID in %s", path)
	return 0
}

func waitProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process %d survived", pid)
}

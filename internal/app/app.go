package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"

	"github.com/hypertrial/intentci/v2/internal/config"
	"github.com/hypertrial/intentci/v2/internal/repo"
	"github.com/hypertrial/intentci/v2/internal/version"
)

const usage = `Usage:
  intentci
  intentci --all
  intentci init
  intentci version
  intentci --help
`

var shellPath = "/bin/zsh"

func Main(args []string, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	var signalCode atomic.Int32
	done := make(chan struct{})
	go func() {
		select {
		case received := <-signals:
			signalCode.Store(int32(exitCodeForSignal(received)))
			cancel()
		case <-done:
		}
	}()

	code := Run(ctx, args, stdout, stderr)
	close(done)
	if interrupted := signalCode.Load(); interrupted != 0 {
		return int(interrupted)
	}
	return code
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "intentci:", err)
		return 2
	}
	return RunFrom(ctx, cwd, args, stdout, stderr)
}

func RunFrom(ctx context.Context, start string, args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 1 && args[0] == "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case len(args) == 1 && args[0] == "version":
		fmt.Fprintln(stdout, version.String())
		return 0
	}
	all := len(args) == 1 && args[0] == "--all"
	initMode := len(args) == 1 && args[0] == "init"
	if len(args) != 0 && !all && !initMode {
		fmt.Fprintln(stderr, "intentci: unsupported arguments:", strings.Join(args, " "))
		fmt.Fprint(stderr, usage)
		return 2
	}

	root, err := repo.Root(start)
	if err != nil {
		fmt.Fprintln(stderr, "intentci:", err)
		return 2
	}
	if initMode {
		if err := initialize(root); err != nil {
			fmt.Fprintln(stderr, "intentci:", err)
			return 2
		}
		fmt.Fprintln(stdout, "Created", filepath.Join(root, config.FileName))
		return 0
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintln(stderr, "intentci:", err)
		return 2
	}

	checks := cfg.Checks
	if !all {
		files, err := repo.Changed(root)
		if err != nil {
			fmt.Fprintln(stderr, "intentci:", err)
			return 2
		}
		if len(files) == 0 {
			fmt.Fprintln(stdout, "No changes; nothing to run.")
			return 0
		}
		checks = selectChecks(cfg.Checks, files)
		if len(checks) == 0 {
			fmt.Fprintf(stdout, "No checks match %d changed file(s).\n", len(files))
			return 0
		}
		fmt.Fprintf(stdout, "%d changed file(s); %d check(s) selected.\n", len(files), len(checks))
	}

	for _, check := range checks {
		fmt.Fprintf(stdout, "\nRUN %s — %s\n$ %s\n", check.ID, check.Intent, check.Run)
		run := `export PATH="$INTENTCI_INHERITED_PATH:$PATH"; ` + check.Run
		command := exec.CommandContext(ctx, shellPath, "-lc", run)
		command.Dir = root
		command.Env = append(os.Environ(), "INTENTCI_INHERITED_PATH="+os.Getenv("PATH"))
		command.Stdout = stdout
		command.Stderr = stderr
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		command.Cancel = func() error {
			err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		command.WaitDelay = time.Second
		err := command.Run()
		if ctx.Err() != nil {
			fmt.Fprintf(stderr, "INTERRUPTED %s\n", check.ID)
			return 130
		}
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				fmt.Fprintf(stderr, "FAIL %s (exit %d)\n", check.ID, exitError.ExitCode())
				return 1
			}
			fmt.Fprintf(stderr, "intentci: start %s: %v\n", check.ID, err)
			return 2
		}
		fmt.Fprintf(stdout, "PASS %s\n", check.ID)
	}
	fmt.Fprintf(stdout, "\nPASS %d check(s)\n", len(checks))
	return 0
}

func exitCodeForSignal(received os.Signal) int {
	if received == syscall.SIGTERM {
		return 143
	}
	return 130
}

func selectChecks(checks []config.Check, files []string) []config.Check {
	for _, file := range files {
		if file == config.FileName {
			return append([]config.Check(nil), checks...)
		}
	}
	var selected []config.Check
	for _, check := range checks {
		matched := false
		for _, pattern := range check.Paths {
			for _, file := range files {
				matched, _ = doublestar.Match(pattern, file)
				if matched {
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			selected = append(selected, check)
		}
	}
	return selected
}

func initialize(root string) error {
	target := filepath.Join(root, config.FileName)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists", config.FileName)
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ".intentci", "config.yaml")); err == nil {
		return fmt.Errorf("v1 configuration found; migrate it using docs/migration-v1-to-v2.md")
	} else if !os.IsNotExist(err) {
		return err
	}

	generated := config.Config{Version: 2, Checks: []config.Check{detect(root)}}
	data, err := yaml.Marshal(generated)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(root, ".intentci-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Link(temporary, target); err != nil {
		return fmt.Errorf("create %s: %w", config.FileName, err)
	}
	return nil
}

func detect(root string) config.Check {
	exists := func(name string) bool {
		info, err := os.Stat(filepath.Join(root, name))
		return err == nil && !info.IsDir()
	}
	if exists("go.mod") {
		return config.Check{
			ID: "go-tests", Intent: "Go changes must keep tests passing.",
			Paths: []string{"**/*.go", "go.mod", "go.sum"}, Run: "go test ./...",
		}
	}
	if exists("package.json") {
		run := "npm test"
		if exists("pnpm-lock.yaml") {
			run = "pnpm test"
		} else if exists("yarn.lock") {
			run = "yarn test"
		}
		return config.Check{
			ID: "node-tests", Intent: "Node changes must keep tests passing.",
			Paths: []string{
				"**/*.js", "**/*.mjs", "**/*.cjs", "**/*.ts", "**/*.tsx",
				"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
			},
			Run: run,
		}
	}
	if exists("pyproject.toml") {
		run := "python3 -m pytest -q"
		if exists("uv.lock") {
			run = "uv run pytest -q"
		}
		return config.Check{
			ID: "python-tests", Intent: "Python changes must keep tests passing.",
			Paths: []string{"**/*.py", "pyproject.toml", "requirements*.txt", "uv.lock"},
			Run:   run,
		}
	}
	if exists("Cargo.toml") {
		return config.Check{
			ID: "rust-tests", Intent: "Rust changes must keep tests passing.",
			Paths: []string{"**/*.rs", "Cargo.toml", "Cargo.lock"}, Run: "cargo test",
		}
	}
	if exists("pom.xml") {
		run := "mvn test"
		if exists("mvnw") {
			run = "./mvnw test"
		}
		return config.Check{
			ID: "java-tests", Intent: "Java changes must keep tests passing.",
			Paths: []string{"**/*.java", "pom.xml", ".mvn/**", "mvnw", "mvnw.cmd"}, Run: run,
		}
	}
	if exists("build.gradle") || exists("build.gradle.kts") ||
		exists("settings.gradle") || exists("settings.gradle.kts") || exists("gradlew") {
		run := "gradle test"
		if exists("gradlew") {
			run = "./gradlew test"
		}
		return config.Check{
			ID: "java-tests", Intent: "Java changes must keep tests passing.",
			Paths: []string{
				"**/*.java", "build.gradle", "build.gradle.kts",
				"settings.gradle", "settings.gradle.kts", "gradle/**", "gradle.lockfile",
				"gradlew", "gradlew.bat",
			},
			Run: run,
		}
	}
	return config.Check{
		ID: "tests", Intent: "Repository changes must pass its configured tests.",
		Paths: []string{"**"},
		Run:   `echo "Edit .intentci.yaml and replace this command." >&2; exit 1`,
	}
}

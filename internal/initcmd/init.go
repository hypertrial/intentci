package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypertrial/intentci/internal/contract"
)

// Result describes what init created.
type Result struct {
	Root         string
	ContractPath string
	Created      []string
}

// Run creates .intentci/ with a starter contract.
func Run(root string) (*Result, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(abs, contract.DirName)
	if err := os.MkdirAll(filepath.Join(dir, "changes"), 0o755); err != nil {
		return nil, fmt.Errorf("create .intentci: %w", err)
	}

	res := &Result{Root: abs, ContractPath: contract.Path(abs)}
	gitignore := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		content := "# IntentCI local artifacts\ntmp/\n*.log\n"
		if err := os.WriteFile(gitignore, []byte(content), 0o644); err != nil {
			return nil, err
		}
		res.Created = append(res.Created, gitignore)
	}

	if _, err := os.Stat(res.ContractPath); err == nil {
		return res, fmt.Errorf("contract already exists: %s", res.ContractPath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	name := filepath.Base(abs)
	draft := detectDraftChecks(abs)
	body := renderContract(name, draft)
	if err := os.WriteFile(res.ContractPath, []byte(body), 0o644); err != nil {
		return nil, err
	}
	res.Created = append(res.Created, res.ContractPath)
	return res, nil
}

type draftCheck struct {
	ID          string
	Description string
	Command     string
	Inputs      []string
}

func detectDraftChecks(root string) []draftCheck {
	var out []draftCheck
	if fileExists(root, "go.mod") {
		out = append(out, draftCheck{
			ID:          "go-test",
			Description: "Go unit tests",
			Command:     "go test ./...",
			Inputs:      []string{"**/*.go", "go.mod", "go.sum"},
		})
	}
	if fileExists(root, "package.json") {
		out = append(out, draftCheck{
			ID:          "npm-test",
			Description: "JavaScript/TypeScript tests",
			Command:     "npm test",
			Inputs:      []string{"**/*.{js,ts,tsx,jsx}", "package.json", "package-lock.json"},
		})
	}
	if fileExists(root, "pyproject.toml") || fileExists(root, "pytest.ini") || fileExists(root, "setup.py") {
		out = append(out, draftCheck{
			ID:          "pytest",
			Description: "Python tests",
			Command:     "pytest",
			Inputs:      []string{"**/*.py", "pyproject.toml", "pytest.ini"},
		})
	}
	if fileExists(root, "Cargo.toml") {
		out = append(out, draftCheck{
			ID:          "cargo-test",
			Description: "Rust tests",
			Command:     "cargo test",
			Inputs:      []string{"**/*.rs", "Cargo.toml", "Cargo.lock"},
		})
	}
	if len(out) == 0 {
		out = append(out, draftCheck{
			ID:          "unit-tests",
			Description: "Repository unit tests (edit command)",
			Command:     "echo 'configure a real test command'",
			Inputs:      []string{"**/*"},
		})
	}
	return out
}

func fileExists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

func renderContract(name string, drafts []draftCheck) string {
	var b strings.Builder
	b.WriteString("version: 1\n\n")
	b.WriteString("product:\n")
	b.WriteString(fmt.Sprintf("  name: %s\n", name))
	b.WriteString("  purpose: Describe the product purpose.\n")
	b.WriteString("  non_goals: []\n\n")
	b.WriteString("policy:\n")
	b.WriteString("  default_base: origin/main\n")
	b.WriteString("  unknown_blocks: true\n")
	b.WriteString("  unverified_blocks: true\n\n")
	b.WriteString("requirements:\n")
	if len(drafts) > 0 {
		d := drafts[0]
		b.WriteString("  - id: BUILD-001\n")
		b.WriteString("    type: reliability\n")
		b.WriteString("    title: Core tests pass\n")
		b.WriteString("    statement: Repository tests that guard product behavior must pass for affected changes.\n")
		b.WriteString("    status: draft\n")
		b.WriteString("    severity: blocking\n")
		b.WriteString("    applies_to:\n")
		b.WriteString("      include:\n")
		for _, in := range d.Inputs {
			b.WriteString(fmt.Sprintf("        - %q\n", in))
		}
		b.WriteString("    verification:\n")
		b.WriteString("      mode: all\n")
		b.WriteString("      checks:\n")
		b.WriteString(fmt.Sprintf("        - %s\n", d.ID))
		b.WriteString("\n")
	}
	b.WriteString("checks:\n")
	for _, d := range drafts {
		b.WriteString(fmt.Sprintf("  - id: %s\n", d.ID))
		b.WriteString(fmt.Sprintf("    description: %s\n", d.Description))
		b.WriteString(fmt.Sprintf("    command: %s\n", d.Command))
		b.WriteString("    profiles:\n")
		b.WriteString("      - fast\n")
		b.WriteString("      - full\n")
		b.WriteString("    inputs:\n")
		for _, in := range d.Inputs {
			b.WriteString(fmt.Sprintf("      - %q\n", in))
		}
		b.WriteString("    timeout: 15m\n")
		b.WriteString("    cache: success\n\n")
	}
	b.WriteString("# Promote requirements from draft to approved when ready.\n")
	b.WriteString("# Only approved requirements affect verification results.\n")
	return b.String()
}

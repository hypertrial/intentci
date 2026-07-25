package changespec

import (
	"fmt"
	"os"
	"strings"

	"github.com/hypertrial/intentci/internal/contract"
)

// Create writes a draft Change Spec scaffold.
func Create(root, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("change id is required")
	}
	path := Path(root, id)
	if _, err := pathStat(path); err == nil {
		return "", fmt.Errorf("change spec already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := mkdirAll(Dir(root), 0o755); err != nil {
		return "", err
	}
	checkID := defaultCheckID(root)
	body := renderScaffold(id, checkID)
	if err := writeFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func defaultCheckID(root string) string {
	c, _, err := contract.LoadFromRoot(root)
	if err != nil || len(c.Checks) == 0 {
		return "unit-tests"
	}
	return c.Checks[0].ID
}

func renderScaffold(id, checkID string) string {
	var b strings.Builder
	b.WriteString("version: 1\n\n")
	b.WriteString(fmt.Sprintf("id: %s\n", id))
	b.WriteString("status: draft\n")
	b.WriteString("type: feature\n")
	b.WriteString(fmt.Sprintf("summary: Describe the intent of %s.\n\n", id))
	b.WriteString("goals:\n")
	b.WriteString("  - Describe the primary goal.\n\n")
	b.WriteString("non_goals:\n")
	b.WriteString("  - Describe an explicit non-goal.\n\n")
	b.WriteString("acceptance:\n")
	b.WriteString("  - id: AC-001\n")
	b.WriteString("    statement: Describe the acceptance criterion.\n")
	b.WriteString("    severity: blocking\n")
	b.WriteString("    verification:\n")
	b.WriteString("      checks:\n")
	b.WriteString(fmt.Sprintf("        - %s\n\n", checkID))
	b.WriteString("affected_requirements: []\n")
	b.WriteString("required_checks: []\n")
	b.WriteString("waivers: []\n")
	return b.String()
}

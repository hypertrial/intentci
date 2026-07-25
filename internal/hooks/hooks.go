package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	beginMarker = "# BEGIN INTENTCI"
	endMarker   = "# END INTENTCI"
)

// intentciBlock is the managed pre-push section.
const intentciBlock = beginMarker + `
# IntentCI managed pre-push hook. Bypass with: git push --no-verify
if ! command -v intentci >/dev/null 2>&1; then
  echo "intentci not found on PATH; install IntentCI or uninstall the hook" >&2
  exit 1
fi
intentci verify --attest
` + endMarker + "\n"

// Install composes the IntentCI block into .git/hooks/pre-push.
func Install(root string) (string, error) {
	hookPath, err := prePushPath(root)
	if err != nil {
		return "", err
	}
	if err := mkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return "", err
	}
	existing, err := readFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var body string
	if os.IsNotExist(err) || len(existing) == 0 {
		body = "#!/usr/bin/env bash\nset -euo pipefail\n\n" + intentciBlock
	} else {
		text := string(existing)
		if strings.Contains(text, beginMarker) && strings.Contains(text, endMarker) {
			updated, ok := replaceBlock(text)
			if !ok {
				return "", fmt.Errorf("malformed IntentCI markers in %s", hookPath)
			}
			body = updated
		} else {
			return "", fmt.Errorf("existing pre-push hook at %s is not IntentCI-managed; compose manually or remove it before install", hookPath)
		}
	}
	if err := writeFile(hookPath, []byte(body), 0o755); err != nil {
		return "", err
	}
	return hookPath, nil
}

// Uninstall removes only the IntentCI-managed section.
func Uninstall(root string) (string, error) {
	hookPath, err := prePushPath(root)
	if err != nil {
		return "", err
	}
	existing, err := readFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return hookPath, nil
		}
		return "", err
	}
	text := string(existing)
	if !strings.Contains(text, beginMarker) {
		return hookPath, nil
	}
	updated, ok := removeBlock(text)
	if !ok {
		return "", fmt.Errorf("malformed IntentCI markers in %s", hookPath)
	}
	trimmed := strings.TrimSpace(updated)
	if trimmed == "" || trimmed == "#!/usr/bin/env bash\nset -euo pipefail" ||
		trimmed == "#!/usr/bin/env bash\nset -euo pipefail\n" ||
		isOnlyShebang(updated) {
		if err := remove(hookPath); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return hookPath, nil
	}
	if err := writeFile(hookPath, []byte(updated), 0o755); err != nil {
		return "", err
	}
	return hookPath, nil
}

func isOnlyShebang(s string) bool {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#!") || line == "set -euo pipefail" {
			continue
		}
		return false
	}
	return true
}

func prePushPath(root string) (string, error) {
	gitDir, err := resolveGitDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(gitDir, "hooks", "pre-push"), nil
}

func resolveGitDir(root string) (string, error) {
	out, err := runGit(root, "rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir), nil
}

func replaceBlock(text string) (string, bool) {
	start := strings.Index(text, beginMarker)
	end := strings.Index(text, endMarker)
	if start < 0 || end < 0 || end < start {
		return "", false
	}
	end += len(endMarker)
	for end < len(text) && (text[end] == '\n' || text[end] == '\r') {
		end++
	}
	return text[:start] + intentciBlock + text[end:], true
}

func removeBlock(text string) (string, bool) {
	start := strings.Index(text, beginMarker)
	end := strings.Index(text, endMarker)
	if start < 0 || end < 0 || end < start {
		return "", false
	}
	end += len(endMarker)
	for end < len(text) && (text[end] == '\n' || text[end] == '\r') {
		end++
	}
	// Also trim a blank line before the block when present.
	for start > 0 && (text[start-1] == '\n' || text[start-1] == '\r') {
		start--
		if start > 0 && text[start-1] == '\n' {
			break
		}
	}
	return text[:start] + text[end:], true
}

var (
	mkdirAll  = os.MkdirAll
	writeFile = os.WriteFile
	readFile  = os.ReadFile
	remove    = os.Remove
	runGit    = func(root string, args ...string) ([]byte, error) {
		cmd := exec.Command("git", append([]string{}, args...)...)
		cmd.Dir = root
		return cmd.Output()
	}
)

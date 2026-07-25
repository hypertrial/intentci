package trust

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// StorePath returns the trusted-repos file path.
func StorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "intentci", "trusted-repos"), nil
}

// IsTrusted reports whether root is trusted.
func IsTrusted(root string) (bool, error) {
	path, err := StorePath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	key := keyFor(root)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == key || line == root {
			return true, nil
		}
	}
	return false, nil
}

// Trust marks root as trusted.
func Trust(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	path, err := StorePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ok, err := IsTrusted(abs)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\t%s\n", keyFor(abs), abs)
	return err
}

// Ensure prompts or auto-trusts before executing repository commands.
func Ensure(root string, autoTrust bool, stdin io.Reader, stderr io.Writer) error {
	ok, err := IsTrusted(root)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if autoTrust {
		fmt.Fprintf(stderr, "Trusting repository for local check execution: %s\n", root)
		return Trust(root)
	}
	fmt.Fprintf(stderr, `
IntentCI will execute repository-defined commands in:
  %s

This is equivalent to running that repository's build or test scripts.
Trust this repository? [y/N]: `, root)

	if stdin == nil {
		stdin = os.Stdin
	}
	reader := bufio.NewReader(stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return fmt.Errorf("repository not trusted; re-run with --trust to allow command execution")
	}
	return Trust(root)
}

func keyFor(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:8])
}

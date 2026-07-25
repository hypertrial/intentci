package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypertrial/intentci/internal/contract"
)

// FindRoot walks upward from start looking for .intentci or .git.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if hasIntentCI(dir) || hasGit(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository root from %s", start)
		}
		dir = parent
	}
}

func hasIntentCI(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, contract.DirName))
	return err == nil && info.IsDir()
}

func hasGit(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// IntentCIDir returns root/.intentci.
func IntentCIDir(root string) string {
	return filepath.Join(root, contract.DirName)
}

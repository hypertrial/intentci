package security

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DefaultProtected are paths agents must not modify during repair (v1.md §23.2).
var DefaultProtected = []string{
	".intentci/**",
	".github/workflows/**",
}

// RedactEnv filters environment variable names matching glob patterns.
func RedactEnv(env []string, patterns []string) []string {
	if len(patterns) == 0 {
		return env
	}
	var out []string
	for _, e := range env {
		name, _, _ := strings.Cut(e, "=")
		if matchAny(patterns, name) {
			out = append(out, name+"=[REDACTED]")
			continue
		}
		out = append(out, e)
	}
	return out
}

func matchAny(patterns []string, s string) bool {
	for _, p := range patterns {
		ok, err := doublestar.Match(p, s)
		if err == nil && ok {
			return true
		}
		// also support simple substring wildcards already in doublestar
	}
	return false
}

// ProtectedViolation returns changed protected paths.
func ProtectedViolation(changed []string, allowRequirementChanges bool, extraProtected []string) []string {
	patterns := append([]string{}, DefaultProtected...)
	patterns = append(patterns, extraProtected...)
	var hits []string
	for _, f := range changed {
		f = filepath.ToSlash(f)
		if allowRequirementChanges && isIntentciConfigPath(f) {
			continue
		}
		for _, p := range patterns {
			ok, err := doublestar.Match(filepath.ToSlash(p), f)
			if err == nil && ok {
				hits = append(hits, f)
				break
			}
		}
	}
	return hits
}

func isIntentciConfigPath(f string) bool {
	f = filepath.ToSlash(f)
	return f == ".intentci/config.yaml" ||
		strings.HasPrefix(f, ".intentci/requirements/") ||
		strings.HasPrefix(f, ".intentci/schemas/") ||
		strings.HasPrefix(f, ".intentci/policies/")
}

// IsTestPath reports whether a path looks like a test file.
func IsTestPath(p string) bool {
	p = filepath.ToSlash(strings.ToLower(p))
	return strings.Contains(p, "/tests/") ||
		strings.Contains(p, "_test.") ||
		strings.HasPrefix(p, "tests/") ||
		strings.Contains(p, "/test/")
}

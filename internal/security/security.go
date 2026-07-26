package security

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DefaultProtected are paths agents must not modify during repair.
var DefaultProtected = []string{
	".intentci/config.yaml",
	".intentci/requirements/**",
	".intentci/schemas/**",
	".intentci/policies/**",
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
	if allowRequirementChanges {
		filtered := patterns[:0]
		for _, p := range patterns {
			if strings.Contains(p, "requirements") || strings.Contains(p, "config.yaml") {
				continue
			}
			filtered = append(filtered, p)
		}
		patterns = filtered
	}
	var hits []string
	for _, f := range changed {
		f = filepath.ToSlash(f)
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

// IsTestPath reports whether a path looks like a test file.
func IsTestPath(p string) bool {
	p = filepath.ToSlash(strings.ToLower(p))
	return strings.Contains(p, "/tests/") ||
		strings.Contains(p, "_test.") ||
		strings.HasPrefix(p, "tests/") ||
		strings.Contains(p, "/test/")
}

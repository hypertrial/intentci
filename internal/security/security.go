package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// PathViolationError identifies traversal, absolute-path, and symlink escapes.
type PathViolationError struct {
	Message string
}

func (e *PathViolationError) Error() string { return e.Message }

// IsPathViolation reports whether an error represents an unsafe path.
func IsPathViolation(err error) bool {
	var violation *PathViolationError
	return errors.As(err, &violation)
}

func pathViolationf(format string, values ...any) error {
	return &PathViolationError{Message: fmt.Sprintf(format, values...)}
}

var absolutePath = filepath.Abs
var evaluateSymlinks = filepath.EvalSymlinks

// Redactor removes configured environment names and their current values from
// content before it reaches persistent evidence.
type Redactor struct {
	replacements []string
	names        []string
}

// NewRedactor builds a deterministic redactor from NAME=value entries.
func NewRedactor(patterns, environment []string) Redactor {
	var redactor Redactor
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || !matchAny(patterns, name) {
			continue
		}
		redactor.names = append(redactor.names, name)
		if value != "" {
			redactor.replacements = append(redactor.replacements, value)
		}
	}
	sort.Slice(redactor.replacements, func(i, j int) bool {
		return len(redactor.replacements[i]) > len(redactor.replacements[j])
	})
	sort.Strings(redactor.names)
	return redactor
}

// Redact replaces secret values and conventional NAME=value renderings.
func (r Redactor) Redact(content string) string {
	for _, value := range r.replacements {
		content = strings.ReplaceAll(content, value, "[REDACTED]")
	}
	for _, name := range r.names {
		for _, separator := range []string{"=", ": "} {
			prefix := name + separator
			start := 0
			for {
				index := strings.Index(content[start:], prefix)
				if index < 0 {
					break
				}
				index += start + len(prefix)
				end := index
				for end < len(content) && content[end] != '\n' && content[end] != '\r' &&
					content[end] != ' ' && content[end] != '\t' && content[end] != ',' &&
					content[end] != '"' {
					end++
				}
				content = content[:index] + "[REDACTED]" + content[end:]
				start = index + len("[REDACTED]")
			}
		}
	}
	return content
}

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

// BoundaryViolations returns files outside allowed paths or inside forbidden paths.
func BoundaryViolations(changed, allowed, forbidden []string) []string {
	var hits []string
	for _, file := range changed {
		file = filepath.ToSlash(file)
		if matchAny(forbidden, file) || (len(allowed) > 0 && !matchAny(allowed, file)) {
			hits = append(hits, file)
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

// ResolveInside resolves a repository-relative path without permitting traversal
// or symlink escape. Missing final paths are allowed when their parent is safe.
func ResolveInside(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", pathViolationf("path must be repository-relative: %q", relative)
	}
	rootAbs, err := absolutePath(root)
	if err != nil {
		return "", err
	}
	rootReal, err := evaluateSymlinks(rootAbs)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(rootReal, filepath.Clean(relative))
	if !inside(rootReal, candidate) {
		return "", pathViolationf("path escapes repository: %q", relative)
	}
	resolved, err := evaluateSymlinks(candidate)
	if err == nil {
		if !inside(rootReal, resolved) {
			return "", pathViolationf("symlink escapes repository: %q", relative)
		}
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parentPath := filepath.Dir(candidate)
	var parent string
	for {
		parent, err = evaluateSymlinks(parentPath)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parentPath = filepath.Dir(parentPath)
	}
	if !inside(rootReal, parent) {
		return "", pathViolationf("symlink parent escapes repository: %q", relative)
	}
	return candidate, nil
}

func inside(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

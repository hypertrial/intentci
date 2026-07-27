package impact

import (
	"path/filepath"
	"testing"

	"github.com/bmatcuk/doublestar/v4"
)

func FuzzV1PathMatching(f *testing.F) {
	f.Add("src/**/*.go", "src/pkg/file.go")
	f.Add("[", "src/file.go")
	f.Add("docs/**", `docs\v1.md`)
	f.Fuzz(func(t *testing.T, pattern, file string) {
		if len(pattern) > 512 || len(file) > 512 {
			t.Skip()
		}
		pattern = filepath.ToSlash(pattern)
		file = filepath.ToSlash(file)
		matched, err := doublestar.Match(pattern, file)
		want := err == nil && matched
		if got := PathMatches([]string{pattern}, file); got != want {
			t.Fatalf("PathMatches(%q, %q)=%t, want %t", pattern, file, got, want)
		}
	})
}

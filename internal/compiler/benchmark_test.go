package compiler_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/compiler"
)

func BenchmarkV1Compile100Requirements(b *testing.B) {
	benchmarkCompilation(b, 100)
}

func BenchmarkV1Compile1000Requirements(b *testing.B) {
	benchmarkCompilation(b, 1000)
}

func benchmarkCompilation(b *testing.B, count int) {
	root := b.TempDir()
	requirements := filepath.Join(root, ".intentci", "requirements")
	if err := os.MkdirAll(requirements, 0o755); err != nil {
		b.Fatal(err)
	}
	configBody := `version: 1
project:
  name: benchmark
requirements:
  paths:
    - .intentci/requirements/**/*.md
`
	if err := os.WriteFile(filepath.Join(root, ".intentci", "config.yaml"), []byte(configBody), 0o644); err != nil {
		b.Fatal(err)
	}
	for index := 0; index < count; index++ {
		id := fmt.Sprintf("REQ-%04d", index)
		requirement := fmt.Sprintf(`---
id: %s
title: Benchmark requirement %d
status: active
priority: required
owners: [benchmark]
applies_to:
  paths: ["src/%04d/**"]
---
# Intent
Keep benchmark behavior correct.
# Obligations
`+"```yaml"+`
- id: OBL-001
  statement: Benchmark evidence exists.
  required: true
  verify:
    provider: command
    id: check
    run: "printf 'intentci-ok\n'"
    result:
      type: exit_code
      equals: 0
      stdout:
        contains: intentci-ok
`+"```"+`
`, id, index, index)
		if err := os.WriteFile(filepath.Join(requirements, id+".md"), []byte(requirement), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, err := compiler.Compile(compiler.Options{Root: root, Strict: true}); err != nil {
			b.Fatal(err)
		}
	}
}

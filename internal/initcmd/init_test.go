package initcmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/initcmd"
)

func TestRun_GoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := initcmd.Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) == 0 {
		t.Fatal("expected created files")
	}
	data, err := os.ReadFile(res.ContractPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "go-test") {
		t.Fatalf("contract=%s", data)
	}
	if _, err := initcmd.Run(dir); err == nil {
		t.Fatal("expected already exists")
	}
}

func TestRun_Detectors(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"package.json", "npm-test"},
		{"pyproject.toml", "pytest"},
		{"Cargo.toml", "cargo-test"},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, tc.file), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := initcmd.Run(dir)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(res.ContractPath)
		if !strings.Contains(string(data), tc.want) {
			t.Fatalf("%s missing %s in %s", tc.file, tc.want, data)
		}
	}
	dir := t.TempDir()
	res, err := initcmd.Run(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(res.ContractPath)
	if !strings.Contains(string(data), "unit-tests") {
		t.Fatalf("fallback missing: %s", data)
	}
}

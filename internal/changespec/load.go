package changespec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/hypertrial/intentci/internal/contract"
)

// DirName is the changes directory under .intentci.
const DirName = "changes"

// Path returns .intentci/changes/<id>.yaml under root.
func Path(root, id string) string {
	return filepath.Join(root, contract.DirName, DirName, id+".yaml")
}

// Dir returns the changes directory.
func Dir(root string) string {
	return filepath.Join(root, contract.DirName, DirName)
}

// Load reads a Change Spec by id.
func Load(root, id string) (*Spec, []byte, error) {
	path := Path(root, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read change spec %q: %w", id, err)
	}
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, nil, fmt.Errorf("parse change spec YAML: %w", err)
	}
	if s.ID != id {
		return nil, nil, fmt.Errorf("change spec id %q does not match file id %q", s.ID, id)
	}
	return &s, data, nil
}

// Hash returns sha256 of raw bytes.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ToJSONMap converts for schema validation.
func ToJSONMap(s *Spec) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	b, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// LoadBase reads the Change Spec content from a git ref when present.
func LoadBase(root, baseRef, id string) ([]byte, bool, error) {
	rel := filepath.ToSlash(filepath.Join(contract.DirName, DirName, id+".yaml"))
	cmd := gitShow(root, baseRef+":"+rel)
	out, err := cmd()
	if err != nil {
		return nil, false, nil
	}
	return out, true, nil
}

// gitShow is overridable for tests.
var gitShow = func(root, object string) func() ([]byte, error) {
	return func() ([]byte, error) {
		return runGit(root, "show", object)
	}
}

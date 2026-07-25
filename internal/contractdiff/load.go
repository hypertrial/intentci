package contractdiff

import (
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/hypertrial/intentci/internal/contract"
)

// LoadBase reads the Product Contract from a git commit when present.
func LoadBase(root, mergeBaseFull string) (*contract.Contract, []byte, bool, error) {
	if mergeBaseFull == "" {
		return nil, nil, false, nil
	}
	rel := filepath.ToSlash(filepath.Join(contract.DirName, contract.ContractFile))
	out, err := gitShow(root, mergeBaseFull+":"+rel)()
	if err != nil {
		return nil, nil, false, nil
	}
	var c contract.Contract
	if err := yaml.Unmarshal(out, &c); err != nil {
		return nil, out, false, fmt.Errorf("parse base contract: %w", err)
	}
	return &c, out, true, nil
}

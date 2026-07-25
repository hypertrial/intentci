package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DirName is the IntentCI config directory.
	DirName = ".intentci"
	// ContractFile is the Product Contract filename.
	ContractFile = "contract.yaml"
)

// Path returns the contract path under root.
func Path(root string) string {
	return filepath.Join(root, DirName, ContractFile)
}

// Load reads and parses a Product Contract from path.
func Load(path string) (*Contract, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read contract: %w", err)
	}
	var c Contract
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, nil, fmt.Errorf("parse contract YAML: %w", err)
	}
	return &c, data, nil
}

// LoadFromRoot loads the contract from root/.intentci/contract.yaml.
func LoadFromRoot(root string) (*Contract, []byte, error) {
	return Load(Path(root))
}

// Hash returns a sha256 hash of the raw contract bytes.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ToJSONMap converts the contract to a generic map for JSON Schema validation.
func ToJSONMap(c *Contract) (map[string]any, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

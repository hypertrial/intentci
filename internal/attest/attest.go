package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/version"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Attestation is a compact PASS-only verification record.
type Attestation struct {
	SchemaVersion       int               `json:"schema_version"`
	IntentCIVersion     string            `json:"intentci_version"`
	Status              string            `json:"status"`
	BaseCommit          string            `json:"base_commit"`
	HeadCommit          string            `json:"head_commit"`
	ContractHash        string            `json:"contract_hash"`
	ChangeSpecHash      string            `json:"change_spec_hash,omitempty"`
	CheckDefinitionHash string            `json:"check_definition_hash"`
	EnvironmentHash     string            `json:"environment_hash"`
	WorkingTreeDirty    bool              `json:"working_tree_dirty"`
	CompletedAt         string            `json:"completed_at"`
	Checks              []CheckAttestation `json:"checks"`
}

// CheckAttestation is a per-check hash record (no stdout/stderr).
type CheckAttestation struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	ResultHash string `json:"result_hash"`
}

// Path returns .intentci/tmp/attestation-<head>.json.
func Path(root, headCommit string) string {
	return filepath.Join(root, contract.DirName, "tmp", "attestation-"+headCommit+".json")
}

// Build creates an attestation from a PASS result.
func Build(res *protocol.Result, checks map[string]contract.Check, envInclude []string) (*Attestation, error) {
	if res == nil {
		return nil, fmt.Errorf("nil result")
	}
	if res.Status != protocol.StatusPass {
		return nil, fmt.Errorf("attestation requires pass status, got %s", res.Status)
	}
	var changeHash string
	if res.ChangeSpec != nil {
		changeHash = res.ChangeSpec.Hash
	}
	attChecks := make([]CheckAttestation, 0, len(res.Checks))
	for _, ch := range res.Checks {
		attChecks = append(attChecks, CheckAttestation{
			ID:         ch.ID,
			Status:     ch.Status,
			ResultHash: hashCheckResult(ch),
		})
	}
	sort.Slice(attChecks, func(i, j int) bool { return attChecks[i].ID < attChecks[j].ID })
	return &Attestation{
		SchemaVersion:       1,
		IntentCIVersion:     version.String(),
		Status:              res.Status,
		BaseCommit:          res.BaseCommit,
		HeadCommit:          res.HeadCommit,
		ContractHash:        res.ContractHash,
		ChangeSpecHash:      changeHash,
		CheckDefinitionHash: hashCheckDefs(checks),
		EnvironmentHash:     hashEnv(envInclude),
		WorkingTreeDirty:    res.WorkingTreeDirty,
		CompletedAt:         time.Now().UTC().Format(time.RFC3339),
		Checks:              attChecks,
	}, nil
}

// Write writes the attestation JSON file.
func Write(root string, att *Attestation) (string, error) {
	if att == nil {
		return "", fmt.Errorf("nil attestation")
	}
	if att.Status != protocol.StatusPass {
		return "", fmt.Errorf("refusing to write non-pass attestation")
	}
	path := Path(root, att.HeadCommit)
	if err := mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := marshalIndent(att, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func hashCheckResult(ch protocol.CheckResult) string {
	payload := map[string]any{
		"id":          ch.ID,
		"status":      ch.Status,
		"exit_code":   ch.ExitCode,
		"duration_ms": ch.DurationMS,
		"reason":      ch.Reason,
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashCheckDefs(checks map[string]contract.Check) string {
	ids := make([]string, 0, len(checks))
	for id := range checks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	type def struct {
		ID        string   `json:"id"`
		Command   string   `json:"command"`
		Timeout   string   `json:"timeout"`
		DependsOn []string `json:"depends_on"`
		Inputs    []string `json:"inputs"`
	}
	var defs []def
	for _, id := range ids {
		ch := checks[id]
		defs = append(defs, def{
			ID: ch.ID, Command: ch.Command, Timeout: ch.Timeout,
			DependsOn: ch.DependsOn, Inputs: ch.Inputs,
		})
	}
	b, _ := json.Marshal(defs)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashEnv(include []string) string {
	vals := map[string]string{}
	for _, k := range include {
		vals[k] = os.Getenv(k)
	}
	b, _ := json.Marshal(vals)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

var (
	mkdirAll      = os.MkdirAll
	writeFile     = os.WriteFile
	marshalIndent = json.MarshalIndent
)

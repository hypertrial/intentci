package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/verdict"
)

// Bundle is a persisted verification run.
type Bundle struct {
	RunID        string                     `json:"run_id"`
	CreatedAt    time.Time                  `json:"created_at"`
	Root         string                     `json:"root"`
	BaseCommit   string                     `json:"base_commit,omitempty"`
	HeadCommit   string                     `json:"head_commit,omitempty"`
	ConfigHash   string                     `json:"config_hash,omitempty"`
	IRHash       string                     `json:"ir_hash,omitempty"`
	Document     *ir.Document               `json:"document,omitempty"`
	Run          verdict.RunResult          `json:"run"`
	ProviderLogs map[string]provider.Result `json:"provider_results,omitempty"`
	Unmapped     []string                   `json:"unmapped_files,omitempty"`
}

// Store writes and loads evidence bundles.
type Store struct {
	Root string // absolute .intentci/runs
}

var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile

// NewStore creates a store under evidence directory relative to repo root.
func NewStore(repoRoot, evidenceDir string) (*Store, error) {
	dir := evidenceDir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, evidenceDir)
	}
	if err := mkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Root: dir}, nil
}

// NewRunID returns a new ULID run id.
func NewRunID() string {
	return ulid.Make().String()
}

// Dir returns the directory for a run.
func (s *Store) Dir(runID string) string {
	return filepath.Join(s.Root, runID)
}

// WriteBundle writes result.json and optional IR.
func (s *Store) WriteBundle(b *Bundle) error {
	dir := s.Dir(b.RunID)
	if err := mkdirAll(dir, 0o755); err != nil {
		return err
	}
	if b.Document != nil {
		raw, err := json.MarshalIndent(b.Document, "", "  ")
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, "compiled-intent.json"), append(raw, '\n'), 0o644); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "result.json"), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	// latest pointer
	latest := filepath.Join(s.Root, "latest")
	_ = writeFile(latest, []byte(b.RunID+"\n"), 0o644)
	return nil
}

// LoadLatest loads the latest bundle if present.
func (s *Store) LoadLatest() (*Bundle, error) {
	data, err := os.ReadFile(filepath.Join(s.Root, "latest"))
	if err != nil {
		return nil, err
	}
	id := string(bytesTrim(data))
	return s.Load(id)
}

// Load loads a bundle by run id.
func (s *Store) Load(runID string) (*Bundle, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir(runID), "result.json"))
	if err != nil {
		return nil, err
	}
	var b Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}
	return &b, nil
}

func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

// WriteRepairPacket writes repair-packet.json into the run directory.
func (s *Store) WriteRepairPacket(runID string, packet any) error {
	dir := s.Dir(runID)
	if err := mkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "repair-packet.json"), append(raw, '\n'), 0o644)
}

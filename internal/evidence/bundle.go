package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	repogit "github.com/hypertrial/intentci/internal/git"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/security"
	"github.com/hypertrial/intentci/internal/verdict"
	appschema "github.com/hypertrial/intentci/pkg/schema"
)

// Bundle is a persisted verification run.
type Bundle struct {
	RunID            string                     `json:"run_id"`
	AttemptID        string                     `json:"attempt_id,omitempty"`
	CreatedAt        time.Time                  `json:"created_at"`
	Root             string                     `json:"root"`
	BaseCommit       string                     `json:"base_commit,omitempty"`
	HeadCommit       string                     `json:"head_commit,omitempty"`
	ConfigHash       string                     `json:"config_hash,omitempty"`
	IRHash           string                     `json:"ir_hash,omitempty"`
	ManifestHash     string                     `json:"manifest_hash,omitempty"`
	Interrupted      bool                       `json:"interrupted,omitempty"`
	Document         *ir.Document               `json:"document,omitempty"`
	VerificationPlan *ir.VerificationPlan       `json:"verification_plan,omitempty"`
	Run              verdict.RunResult          `json:"run"`
	ProviderLogs     map[string]provider.Result `json:"provider_results,omitempty"`
	RepositoryState  *repogit.State             `json:"repository_state,omitempty"`
	Unmapped         []string                   `json:"unmapped_files,omitempty"`
}

// Manifest records the immutable artifacts belonging to a run.
type Manifest struct {
	SchemaVersion string             `json:"schema_version"`
	RunID         string             `json:"run_id"`
	HashAlgorithm string             `json:"hash_algorithm"`
	Artifacts     []ManifestArtifact `json:"artifacts"`
}

// ManifestArtifact is a content-addressed run artifact.
type ManifestArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// FinalVerdict closes a run without introducing a manifest hash cycle.
type FinalVerdict struct {
	SchemaVersion string            `json:"schema_version"`
	RunID         string            `json:"run_id"`
	ManifestHash  string            `json:"manifest_hash"`
	Run           verdict.RunResult `json:"run"`
	Interrupted   bool              `json:"interrupted,omitempty"`
}

// Store writes and loads evidence bundles.
type Store struct {
	Root           string // absolute configured evidence directory
	RedactPatterns []string
	repoRoot       string
	relativeRoot   string
}

var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile
var renameFile = os.Rename
var removeFile = os.Remove
var readFile = os.ReadFile
var statFile = os.Stat
var absolutePath = filepath.Abs
var evaluateSymlinks = filepath.EvalSymlinks
var relativePath = filepath.Rel

// NewStore creates a store under evidence directory relative to repo root.
func NewStore(repoRoot, evidenceDir string) (*Store, error) {
	if evidenceDir == "" {
		return nil, fmt.Errorf("evidence directory is required")
	}
	dir := evidenceDir
	relativeRoot := ""
	if !filepath.IsAbs(dir) {
		relativeRoot = filepath.Clean(evidenceDir)
		resolved, err := security.ResolveInside(repoRoot, evidenceDir)
		if err != nil {
			return nil, err
		}
		if err := mkdirAll(resolved, 0o755); err != nil {
			return nil, err
		}
		dir = filepath.Join(repoRoot, evidenceDir)
	}
	if err := mkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	absolute, err := absolutePath(dir)
	if err != nil {
		return nil, err
	}
	if _, err := evaluateSymlinks(absolute); err != nil {
		return nil, err
	}
	return &Store{Root: absolute, repoRoot: repoRoot, relativeRoot: relativeRoot}, nil
}

// NewRunID returns a new ULID run id.
func NewRunID() string {
	return ulid.Make().String()
}

// Dir returns the directory for a run.
func (s *Store) Dir(runID string) string {
	return filepath.Join(s.Root, runID)
}

// WriteBundle persists a complete single-attempt run.
func (s *Store) WriteBundle(bundle *Bundle) error {
	if err := s.WriteAttempt(bundle); err != nil {
		return err
	}
	return s.Finalize(bundle)
}

// WriteAttempt writes immutable inputs, evidence, logs, and the attempt verdict.
func (s *Store) WriteAttempt(bundle *Bundle) error {
	if err := validateRunID(bundle.RunID); err != nil {
		return err
	}
	if err := s.ensureOpen(bundle.RunID); err != nil {
		return err
	}
	if bundle.AttemptID == "" {
		bundle.AttemptID = "attempt-001"
	}
	if err := validateAttemptID(bundle.AttemptID); err != nil {
		return err
	}
	if bundle.CreatedAt.IsZero() {
		bundle.CreatedAt = time.Now().UTC()
	}
	if bundle.Run.Requirements == nil {
		bundle.Run.Requirements = []verdict.RequirementResult{}
	}
	if err := validateVerdicts(bundle.Run); err != nil {
		return err
	}
	runDir := s.Dir(bundle.RunID)
	if _, err := s.safePath(runDir); err != nil {
		return err
	}
	if err := mkdirAll(runDir, 0o755); err != nil {
		return err
	}
	attemptDir := filepath.Join(runDir, "attempts", bundle.AttemptID)
	if err := mkdirAll(filepath.Join(attemptDir, "logs"), 0o755); err != nil {
		return err
	}
	if err := mkdirAll(filepath.Join(attemptDir, "artifacts"), 0o755); err != nil {
		return err
	}
	if bundle.Document != nil {
		if bundle.Document.Requirements == nil {
			bundle.Document.Requirements = make([]ir.Requirement, 0)
		}
		if bundle.Document.Hash == "" {
			if err := bundle.Document.ComputeHashes(); err != nil {
				return err
			}
			bundle.IRHash = bundle.Document.Hash
		}
		if err := appschema.Validate("ir", bundle.Document); err != nil {
			return err
		}
		if err := s.writeJSONImmutable(filepath.Join(runDir, "compiled-intent.json"), bundle.Document); err != nil {
			return err
		}
		if bundle.VerificationPlan == nil {
			plan, _ := ir.BuildVerificationPlan(bundle.Document, bundle.Document.Requirements)
			bundle.VerificationPlan = plan
		}
	}
	if bundle.VerificationPlan != nil {
		if err := appschema.Validate("plan", bundle.VerificationPlan); err != nil {
			return err
		}
		if err := s.writeJSONImmutable(filepath.Join(runDir, "verification-plan.json"), bundle.VerificationPlan); err != nil {
			return err
		}
	}
	if bundle.RepositoryState != nil {
		if err := s.writeInitialJSON(filepath.Join(runDir, "repository-state.json"), bundle.RepositoryState); err != nil {
			return err
		}
		if err := s.writeJSONImmutable(filepath.Join(attemptDir, "repository-state.json"), bundle.RepositoryState); err != nil {
			return err
		}
	}
	patch := ""
	if bundle.RepositoryState != nil {
		patch = bundle.RepositoryState.DiffPatch
	}
	if err := s.writeInitialImmutable(filepath.Join(runDir, "diff.patch"), []byte(patch)); err != nil {
		return err
	}
	if err := s.writeImmutable(filepath.Join(attemptDir, "diff.patch"), []byte(patch)); err != nil {
		return err
	}

	var records []provider.Evidence
	keys := sortedProviderKeys(bundle.ProviderLogs)
	for _, key := range keys {
		result := bundle.ProviderLogs[key]
		for _, record := range result.Evidence {
			if err := appschema.Validate("evidence", record); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			records = append(records, record)
		}
		name := safeName(key)
		if result.Stdout != "" {
			if err := s.writeImmutable(filepath.Join(attemptDir, "logs", name+".stdout"), []byte(result.Stdout)); err != nil {
				return err
			}
		}
		if result.Stderr != "" {
			if err := s.writeImmutable(filepath.Join(attemptDir, "logs", name+".stderr"), []byte(result.Stderr)); err != nil {
				return err
			}
		}
	}
	if err := s.writeJSONImmutable(filepath.Join(attemptDir, "evidence.json"), records); err != nil {
		return err
	}
	if err := s.writeJSONImmutable(filepath.Join(attemptDir, "verdict.json"), bundle.Run); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	if err := s.writeAtomic(filepath.Join(runDir, "result.json"), append(raw, '\n')); err != nil {
		return err
	}
	return s.writeAtomic(filepath.Join(s.Root, "latest"), []byte(bundle.RunID+"\n"))
}

func validateVerdicts(run verdict.RunResult) error {
	if err := appschema.Validate("verdict", run.Verdict); err != nil {
		return err
	}
	for _, requirement := range run.Requirements {
		if err := appschema.Validate("verdict", requirement.Verdict); err != nil {
			return fmt.Errorf("%s: %w", requirement.ID, err)
		}
		for _, obligation := range requirement.Obligations {
			if err := appschema.Validate("verdict", obligation.Verdict); err != nil {
				return fmt.Errorf("%s/%s: %w", requirement.ID, obligation.ID, err)
			}
		}
	}
	return nil
}

// WriteReport writes a generated report before manifest finalization.
func (s *Store) WriteReport(runID, name string, content []byte) error {
	if err := validateRunID(runID); err != nil {
		return err
	}
	if err := s.ensureOpen(runID); err != nil {
		return err
	}
	switch name {
	case "report.txt", "report.json", "report.junit.xml":
	default:
		return fmt.Errorf("unsupported report path %q", name)
	}
	return s.writeImmutable(filepath.Join(s.Dir(runID), name), content)
}

// Finalize hashes all immutable files and writes the manifest and final verdict.
func (s *Store) Finalize(bundle *Bundle) error {
	if err := validateRunID(bundle.RunID); err != nil {
		return err
	}
	runDir := s.Dir(bundle.RunID)
	safeRunDir, err := s.safePath(runDir)
	if err != nil {
		return err
	}
	artifacts, err := hashArtifacts(safeRunDir)
	if err != nil {
		return err
	}
	manifest := Manifest{
		SchemaVersion: "1.0", RunID: bundle.RunID, HashAlgorithm: "sha256", Artifacts: artifacts,
	}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	raw = append(raw, '\n')
	if err := s.writeImmutable(filepath.Join(runDir, "manifest.json"), raw); err != nil {
		return err
	}
	sum := sha256.Sum256(s.redact(raw))
	bundle.ManifestHash = hex.EncodeToString(sum[:])
	final := FinalVerdict{
		SchemaVersion: "1.0", RunID: bundle.RunID, ManifestHash: bundle.ManifestHash,
		Run: bundle.Run, Interrupted: bundle.Interrupted,
	}
	return s.writeImmutableJSON(filepath.Join(runDir, "final-verdict.json"), final)
}

// LoadLatest loads the latest bundle if present.
func (s *Store) LoadLatest() (*Bundle, error) {
	path, err := s.safePath(filepath.Join(s.Root, "latest"))
	if err != nil {
		return nil, err
	}
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	return s.Load(string(bytesTrim(data)))
}

// Load loads a bundle by run id.
func (s *Store) Load(runID string) (*Bundle, error) {
	if err := validateRunID(runID); err != nil {
		return nil, err
	}
	path, err := s.safePath(filepath.Join(s.Dir(runID), "result.json"))
	if err != nil {
		return nil, err
	}
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}
	return &bundle, nil
}

func bytesTrim(value []byte) []byte {
	return []byte(strings.TrimSpace(string(value)))
}

// WriteRepairPacket writes a compatibility packet at the run root.
func (s *Store) WriteRepairPacket(runID string, packet any) error {
	if err := validateRunID(runID); err != nil {
		return err
	}
	if err := s.ensureOpen(runID); err != nil {
		return err
	}
	return s.writeJSONAtomic(filepath.Join(s.Dir(runID), "repair-packet.json"), packet)
}

// WriteRepairPacketForAttempt writes an immutable packet under an attempt.
func (s *Store) WriteRepairPacketForAttempt(runID, attemptID string, packet any) (string, error) {
	if err := validateRunID(runID); err != nil {
		return "", err
	}
	if err := validateAttemptID(attemptID); err != nil {
		return "", err
	}
	if err := s.ensureOpen(runID); err != nil {
		return "", err
	}
	if err := appschema.Validate("repair", packet); err != nil {
		return "", err
	}
	path := filepath.Join(s.Dir(runID), "attempts", attemptID, "repair-packet.json")
	if err := s.writeImmutableJSON(path, packet); err != nil {
		return "", err
	}
	return path, nil
}

// WriteAgentLog writes a redacted immutable agent stream.
func (s *Store) WriteAgentLog(runID, attemptID, stream string, content []byte) error {
	if err := validateRunID(runID); err != nil {
		return err
	}
	if err := validateAttemptID(attemptID); err != nil {
		return err
	}
	if err := s.ensureOpen(runID); err != nil {
		return err
	}
	if stream != "stdout" && stream != "stderr" {
		return fmt.Errorf("invalid agent stream %q", stream)
	}
	path := filepath.Join(s.Dir(runID), "attempts", attemptID, "logs", "agent."+stream)
	return s.writeImmutable(path, content)
}

// WriteRepairArtifact writes one of the repair controller's immutable records.
func (s *Store) WriteRepairArtifact(runID, attemptID, name string, content []byte) error {
	if err := validateRunID(runID); err != nil {
		return err
	}
	if err := validateAttemptID(attemptID); err != nil {
		return err
	}
	if err := s.ensureOpen(runID); err != nil {
		return err
	}
	switch name {
	case "patch-before.diff", "patch-after.diff", "agent-exit.json":
	default:
		return fmt.Errorf("invalid repair artifact %q", name)
	}
	path := filepath.Join(s.Dir(runID), "attempts", attemptID, name)
	return s.writeImmutable(path, content)
}

func (s *Store) ensureOpen(runID string) error {
	path, err := s.safePath(filepath.Join(s.Dir(runID), "manifest.json"))
	if err != nil {
		return err
	}
	if _, err := statFile(path); err == nil {
		return fmt.Errorf("run %s is finalized and immutable", runID)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) writeJSONImmutable(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return s.writeImmutable(path, append(raw, '\n'))
}

func (s *Store) writeImmutableJSON(path string, value any) error {
	return s.writeJSONImmutable(path, value)
}

func (s *Store) writeJSONAtomic(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return s.writeAtomic(path, append(raw, '\n'))
}

func (s *Store) writeInitialJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return s.writeInitialImmutable(path, append(raw, '\n'))
}

func (s *Store) writeInitialImmutable(path string, content []byte) error {
	safe, err := s.safePath(path)
	if err != nil {
		return err
	}
	if _, err := statFile(safe); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return s.writeImmutable(path, content)
}

func (s *Store) writeImmutable(path string, content []byte) error {
	original := path
	safe, err := s.safePath(path)
	if err != nil {
		return err
	}
	path = safe
	redacted := s.redact(content)
	if existing, err := readFile(path); err == nil {
		if string(existing) == string(redacted) {
			return nil
		}
		return fmt.Errorf("immutable artifact already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return s.writeAtomic(original, redacted)
}

func (s *Store) writeAtomic(path string, content []byte) error {
	safe, err := s.safePath(path)
	if err != nil {
		return err
	}
	path = safe
	if err := mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp-"+ulid.Make().String())
	if err := writeFile(temporary, s.redact(content), 0o644); err != nil {
		return err
	}
	defer removeFile(temporary)
	return renameFile(temporary, path)
}

func (s *Store) safePath(path string) (string, error) {
	relative, err := relativePath(s.Root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("evidence path escapes configured directory: %s", path)
	}
	if s.relativeRoot == "" {
		root, err := evaluateSymlinks(s.Root)
		if err != nil {
			return "", err
		}
		return security.ResolveInside(root, relative)
	}
	root, err := security.ResolveInside(s.repoRoot, s.relativeRoot)
	if err != nil {
		return "", err
	}
	return security.ResolveInside(root, relative)
}

func (s *Store) redact(content []byte) []byte {
	redactor := security.NewRedactor(s.RedactPatterns, os.Environ())
	if json.Valid(content) {
		decoder := json.NewDecoder(strings.NewReader(string(content)))
		decoder.UseNumber()
		var value any
		_ = decoder.Decode(&value)
		value = redactJSONValue(value, redactor)
		raw, _ := json.MarshalIndent(value, "", "  ")
		return append(raw, '\n')
	}
	return []byte(redactor.Redact(string(content)))
}

func redactJSONValue(value any, redactor security.Redactor) any {
	switch typed := value.(type) {
	case string:
		return redactor.Redact(typed)
	case []any:
		for index := range typed {
			typed[index] = redactJSONValue(typed[index], redactor)
		}
	case map[string]any:
		for key := range typed {
			typed[key] = redactJSONValue(typed[key], redactor)
		}
	}
	return value
}

func validateRunID(value string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("invalid run id %q", value)
	}
	return nil
}

func validateAttemptID(value string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("invalid attempt id %q", value)
	}
	return nil
}

func sortedProviderKeys(results map[string]provider.Result) []string {
	keys := make([]string, 0, len(results))
	for key := range results {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func safeName(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "provider"
	}
	return builder.String()
}

func hashArtifacts(root string) ([]ManifestArtifact, error) {
	var artifacts []ManifestArtifact
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		relative = filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("evidence artifact may not be a symlink: %s", relative)
		}
		if entry.IsDir() {
			return nil
		}
		if relative == "manifest.json" || relative == "final-verdict.json" || strings.Contains(relative, ".tmp-") {
			return nil
		}
		content, err := readFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		artifacts = append(artifacts, ManifestArtifact{
			Path: relative, SHA256: hex.EncodeToString(sum[:]),
		})
		return nil
	})
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, err
}

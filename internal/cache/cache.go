package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/runner"
	"github.com/hypertrial/intentci/internal/version"
	"github.com/hypertrial/intentci/pkg/protocol"
)

// Store is a content-addressed success cache.
type Store struct {
	Root string
}

// userCacheDir is overridable in tests.
var userCacheDir = os.UserCacheDir
var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile
var rename = os.Rename
var readFile = os.ReadFile
var remove = os.Remove
var copyFile = io.Copy

// SetUserCacheDir overrides userCacheDir for tests.
func SetUserCacheDir(fn func() (string, error)) func() (string, error) {
	prev := userCacheDir
	userCacheDir = fn
	return prev
}

// DefaultRoot returns ~/.cache/intentci.
func DefaultRoot() (string, error) {
	dir, err := userCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "intentci"), nil
}

// Open returns a store at the default or provided root.
func Open(root string) (*Store, error) {
	if root == "" {
		var err error
		root, err = DefaultRoot()
		if err != nil {
			return nil, err
		}
	}
	if err := mkdirAll(filepath.Join(root, "objects"), 0o755); err != nil {
		return nil, err
	}
	return &Store{Root: root}, nil
}

// KeyInput is material for a cache key.
type KeyInput struct {
	Check        contract.Check
	ContractHash string
	ChangeHash   string
	RepoRoot     string
	EnvInclude   []string
}

// Key builds a content hash for a check. Returns ok=false when caching is disallowed.
func Key(in KeyInput) (string, bool, error) {
	if in.Check.Cache == "off" {
		return "", false, nil
	}
	if len(in.Check.Inputs) == 0 {
		return "", false, nil
	}
	inputHash, err := hashInputs(in.RepoRoot, in.Check.Inputs)
	if err != nil {
		return "", false, err
	}
	envVals := map[string]string{}
	for _, k := range in.EnvInclude {
		envVals[k] = os.Getenv(k)
	}
	payload := map[string]any{
		"intentci_version": version.String(),
		"check_id":         in.Check.ID,
		"command":          in.Check.Command,
		"timeout":          in.Check.Timeout,
		"depends_on":       in.Check.DependsOn,
		"profiles":         in.Check.Profiles,
		"inputs":           in.Check.Inputs,
		"input_hash":       inputHash,
		"contract_hash":    in.ContractHash,
		"change_hash":      in.ChangeHash,
		"os":               runtime.GOOS,
		"arch":             runtime.GOARCH,
		"env":              envVals,
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), true, nil
}

func hashInputs(root string, patterns []string) (string, error) {
	var files []string
	seen := map[string]struct{}{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == ".intentci" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, p := range patterns {
			ok, mErr := doublestar.PathMatch(p, rel)
			if mErr != nil {
				return mErr
			}
			if ok {
				if _, exists := seen[rel]; !exists {
					seen[rel] = struct{}{}
					files = append(files, rel)
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		sum, err := fileSHA(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s %s\n", sum, f)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := copyFile(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type entry struct {
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// Get restores a successful cached result.
func (s *Store) Get(key string) (runner.Result, bool) {
	path := s.objectPath(key)
	data, err := readFile(path)
	if err != nil {
		return runner.Result{}, false
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		_ = remove(path)
		return runner.Result{}, false
	}
	if e.Status != protocol.CheckPass {
		_ = remove(path)
		return runner.Result{}, false
	}
	return runner.Result{
		Status:     e.Status,
		ExitCode:   e.ExitCode,
		DurationMS: e.DurationMS,
		Stdout:     e.Stdout,
		Stderr:     e.Stderr,
		Reason:     e.Reason,
	}, true
}

// Put stores a successful check result.
func (s *Store) Put(key string, res runner.Result) error {
	if res.Status != protocol.CheckPass {
		return nil
	}
	e := entry{
		Status:     res.Status,
		ExitCode:   res.ExitCode,
		DurationMS: res.DurationMS,
		Stdout:     res.Stdout,
		Stderr:     res.Stderr,
		Reason:     res.Reason,
	}
	data, _ := json.Marshal(e)
	tmp := s.objectPath(key) + ".tmp"
	if err := writeFile(tmp, data, 0o644); err != nil {
		return err
	}
	return rename(tmp, s.objectPath(key))
}

// ObjectPath returns the on-disk path for a cache key.
func (s *Store) ObjectPath(key string) string {
	if strings.Contains(key, string(os.PathSeparator)) || strings.Contains(key, "..") {
		key = "invalid"
	}
	return filepath.Join(s.Root, "objects", key+".json")
}

func (s *Store) objectPath(key string) string { return s.ObjectPath(key) }

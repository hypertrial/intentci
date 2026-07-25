package semantic

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestInjectedErrorPaths(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		oldM, oldN, oldR := jsonMarshal, httpNewRequest, ioReadAll
		defer func() { jsonMarshal, httpNewRequest, ioReadAll = oldM, oldN, oldR }()

		p := &HTTPProvider{URL: "http://example.com"}
		jsonMarshal = func(any) ([]byte, error) { return nil, errors.New("marshal") }
		if _, err := p.Analyze(context.Background(), Request{}); err == nil {
			t.Fatal("marshal")
		}
		jsonMarshal = oldM
		httpNewRequest = func(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
			return nil, errors.New("req")
		}
		if _, err := p.Analyze(context.Background(), Request{}); err == nil {
			t.Fatal("req")
		}
		httpNewRequest = oldN

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer srv.Close()
		p = &HTTPProvider{URL: srv.URL, Client: srv.Client()}
		ioReadAll = func(r io.Reader) ([]byte, error) { return nil, errors.New("read") }
		if _, err := p.Analyze(context.Background(), Request{}); err == nil {
			t.Fatal("read")
		}
		ioReadAll = oldR
		if _, err := p.Analyze(context.Background(), Request{}); err == nil {
			t.Fatal("empty 500 body")
		}

		// Do error
		p = &HTTPProvider{URL: "http://127.0.0.1:1", Timeout: 1}
		if _, err := p.Analyze(context.Background(), Request{}); err == nil {
			t.Fatal("do")
		}
	})

	t.Run("local marshal", func(t *testing.T) {
		old := localJSONMarshal
		defer func() { localJSONMarshal = old }()
		localJSONMarshal = func(any) ([]byte, error) { return nil, errors.New("m") }
		p := &LocalProvider{Command: "true"}
		if _, err := p.Analyze(context.Background(), Request{}); err == nil {
			t.Fatal("marshal")
		}
	})

	t.Run("run inject", func(t *testing.T) {
		oldB, oldP, oldE := buildRequest, newProvider, encodeJSON
		defer func() { buildRequest, newProvider, encodeJSON = oldB, oldP, oldE }()

		buildRequest = func(BuildOptions) (Request, error) { return Request{}, errors.New("build") }
		if _, err := Run(context.Background(), RunOptions{Contract: &contract.Contract{Product: contract.Product{Name: "n", Purpose: "p"}}}); err == nil {
			t.Fatal("build")
		}
		buildRequest = func(BuildOptions) (Request, error) { return Request{}, nil }
		encodeJSON = func(io.Writer, any) error { return errors.New("enc") }
		if _, err := Run(context.Background(), RunOptions{
			Contract:          &contract.Contract{Product: contract.Product{Name: "n", Purpose: "p"}},
			ShowSemanticInput: true,
			Stdout:            &bytes.Buffer{},
		}); err == nil {
			t.Fatal("encode")
		}
		encodeJSON = oldE
		newProvider = func(*contract.SemanticProvider) (Provider, error) { return nil, errors.New("np") }
		out, err := Run(context.Background(), RunOptions{
			Contract: &contract.Contract{
				Product: contract.Product{Name: "n", Purpose: "p"},
				Policy: contract.Policy{Semantic: contract.SemanticPolicy{
					Enabled: true, Enforcement: "advisory",
					Provider: &contract.SemanticProvider{Type: "local", Command: "x"},
				}},
			},
		})
		if err != nil || out.ProviderErr == nil {
			t.Fatalf("%v %+v", err, out)
		}
	})
}

func TestRemainingBranches(t *testing.T) {
	// budget exhausted after full-size diff
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b"), []byte("y"), 0o644)
	req, err := BuildRequest(BuildOptions{
		Root: dir,
		Contract: &contract.Contract{Product: contract.Product{Name: "n", Purpose: "p"}},
		ChangedFiles: []string{"a", "b"},
		DiffFn: func(string, string) (string, error) {
			return string(make([]byte, MaxInputBytes+10)), nil
		},
		ReadFile: func(path string) ([]byte, error) {
			return []byte("content"), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Snippets) != 0 {
		t.Fatalf("expected no snippets when budget exhausted, got %d", len(req.Snippets))
	}

	// empty snippet content after redact/truncate
	req, err = BuildRequest(BuildOptions{
		Root:         dir,
		Contract:     &contract.Contract{Product: contract.Product{Name: "n", Purpose: "p"}},
		ChangedFiles: []string{"a"},
		DiffFn:       func(string, string) (string, error) { return "", nil },
		ReadFile:     func(path string) ([]byte, error) { return []byte{}, nil },
	})
	if err != nil || len(req.Snippets) != 0 {
		t.Fatalf("%v %#v", err, req.Snippets)
	}

	oldGit := gitOutput
	defer func() { gitOutput = oldGit }()
	calls := 0
	gitOutput = func(root string, args ...string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("fail head")
		}
		return []byte("diff --git a/f b/f\n"), nil
	}
	out, err := defaultUnifiedDiff(dir, "abc")
	if err != nil || out == "" {
		t.Fatalf("%v %q", err, out)
	}
	gitOutput = func(root string, args ...string) ([]byte, error) { return nil, errors.New("fail") }
	if _, err := defaultUnifiedDiff(dir, "abc"); err == nil {
		t.Fatal("both fail")
	}
	gitOutput = oldGit
	_ = exec.Command

	// semanticMode empty from contract
	c := &contract.Contract{Requirements: []contract.Requirement{{ID: "R", Verification: contract.Verification{Semantic: ""}}}}
	if semanticMode(MergeOptions{Contract: c}, "R") != "optional" {
		t.Fatal()
	}
	// lookup miss
	if _, ok := lookupRequirement(c, "NO"); ok {
		t.Fatal()
	}
	// canBlockingFail no evidence
	if canBlockingFail(&protocol.RequirementResult{ID: "R"}, Finding{Confidence: 1}, "required", "blocking", 0.5, MergeOptions{Contract: &contract.Contract{Requirements: []contract.Requirement{{ID: "R", Status: "approved"}}}}) {
		t.Fatal()
	}
	// MarkUnavailable waived
	_ = MarkUnavailable([]protocol.RequirementResult{{ID: "R", Status: protocol.ReqWaived}}, MergeOptions{SemanticModes: map[string]string{"R": "required"}}, "x")

	// empty requirements slice init
	req, err = BuildRequest(BuildOptions{
		Root:     dir,
		Contract: &contract.Contract{Product: contract.Product{Name: "n", Purpose: "p"}},
		Selection: impact.Selection{Requirements: []impact.SelectedRequirement{{
			Requirement: contract.Requirement{ID: "D", Status: "draft", Verification: contract.Verification{Semantic: "optional"}},
		}}},
		DiffFn: func(string, string) (string, error) { return "", nil },
	})
	if err != nil || req.Requirements == nil {
		t.Fatalf("%v %#v", err, req.Requirements)
	}
}

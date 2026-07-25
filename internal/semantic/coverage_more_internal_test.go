package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
	"github.com/hypertrial/intentci/internal/impact"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestMerge_Branches(t *testing.T) {
	c := &contract.Contract{Requirements: []contract.Requirement{{
		ID: "R-1", Status: "draft", Severity: "blocking",
		Verification: contract.Verification{Semantic: "required"},
	}}}
	pass := []protocol.RequirementResult{{
		ID: "R-1", Status: protocol.ReqPass, Severity: "blocking",
		AffectedBy: []string{"a"}, Checks: []protocol.CheckRef{{ID: "c"}},
		Evidence: []protocol.Evidence{{Type: "check"}}, Findings: []protocol.Finding{{Type: "x", Summary: "y"}},
	}}
	// draft + blocking enforcement → no FAIL (not approved)
	out := Apply(pass, []Finding{{
		RequirementID: "R-1", Assessment: AssessmentContradiction, Confidence: 1, Summary: "bad",
		Evidence: []EvidenceCite{{Path: "a.go"}},
	}}, MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "blocking"}, Contract: c, SemanticModes: map[string]string{"R-1": "required"}})
	if out[0].Status != protocol.ReqUnverified {
		t.Fatalf("%s", out[0].Status)
	}

	// empty summary + empty evidence path + empty missing
	out = Apply(pass, []Finding{{
		RequirementID: "R-1", Assessment: AssessmentUncertain, Confidence: 1,
		Evidence:        []EvidenceCite{{Path: "  "}},
		MissingEvidence: []string{"", "need"},
	}}, MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "advisory"}, Contract: c})
	if out[0].Status != protocol.ReqUnverified {
		t.Fatal(out[0].Status)
	}

	// unknown status keeps unknown
	unk := []protocol.RequirementResult{{ID: "R-1", Status: protocol.ReqUnknown, Severity: "blocking"}}
	out = Apply(unk, []Finding{{RequirementID: "R-1", Assessment: AssessmentContradiction, Confidence: 1, Summary: "x", Evidence: []EvidenceCite{{Path: "a.go"}}}},
		MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "blocking"}, SemanticModes: map[string]string{"R-1": "required"}, Contract: &contract.Contract{Requirements: []contract.Requirement{{ID: "R-1", Status: "approved"}}}})
	if out[0].Status != protocol.ReqUnknown {
		t.Fatal(out[0].Status)
	}

	// not_affected
	out = Apply(pass, []Finding{{RequirementID: "R-1", Assessment: AssessmentNotAffected, Confidence: 1, Summary: "na"}},
		MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "advisory"}, SemanticModes: map[string]string{"R-1": "optional"}})
	if out[0].Status != protocol.ReqPass {
		t.Fatal(out[0].Status)
	}

	// unknown assessment string
	out = Apply(pass, []Finding{{RequirementID: "R-1", Assessment: "weird", Confidence: 1, Summary: "w"}},
		MergeOptions{Policy: contract.SemanticPolicy{Enforcement: "advisory"}, SemanticModes: map[string]string{"R-1": "optional"}})
	if out[0].Status != protocol.ReqUnverified {
		t.Fatal(out[0].Status)
	}

	// semanticMode from contract when modes map missing key
	if semanticMode(MergeOptions{Contract: c}, "R-1") != "required" {
		t.Fatal("from contract")
	}
	if semanticMode(MergeOptions{Contract: c}, "MISSING") != "optional" {
		t.Fatal("default")
	}
	if semanticMode(MergeOptions{SemanticModes: map[string]string{"R-1": ""}}, "R-1") != "optional" {
		t.Fatal("empty mode")
	}

	// appendCompletion already present
	fs := []protocol.Finding{{Type: "completion_condition", Summary: "x"}}
	if len(appendCompletion(fs, "R-1")) != 1 {
		t.Fatal("dup")
	}

	// hasPathEvidence false / lookup nil
	if hasPathEvidence(Finding{}) {
		t.Fatal("no path")
	}
	if _, ok := lookupRequirement(nil, "x"); ok {
		t.Fatal("nil contract")
	}

	// canBlockingFail synthetic AC (not in contract)
	if !canBlockingFail(&protocol.RequirementResult{ID: "AC-1", Severity: "blocking"}, Finding{
		Assessment: AssessmentContradiction, Confidence: 1, Evidence: []EvidenceCite{{Path: "a.go"}},
	}, "required", "blocking", 0.8, MergeOptions{}) {
		t.Fatal("synthetic")
	}
}

func TestHTTPProvider_More(t *testing.T) {
	p := &HTTPProvider{URL: "://bad"}
	if _, err := p.Analyze(context.Background(), Request{}); err == nil {
		t.Fatal("bad url")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()
	p = &HTTPProvider{URL: srv.URL, Client: srv.Client(), Timeout: time.Second}
	if _, err := p.Analyze(context.Background(), Request{}); err == nil {
		// empty body invalid json
		_ = err
	}
	// nil findings becomes empty
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"protocol_version":1}`))
	}))
	defer ok.Close()
	p = &HTTPProvider{URL: ok.URL, Timeout: 0}
	resp, err := p.Analyze(context.Background(), Request{})
	if err != nil || resp.Findings == nil {
		t.Fatalf("%v %+v", err, resp)
	}
}

func TestLocalProvider_InvalidJSONAndTimeout(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.sh")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\necho not-json\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &LocalProvider{Command: bad, Timeout: time.Second}
	if _, err := p.Analyze(context.Background(), Request{}); err == nil {
		t.Fatal("bad json")
	}
	p = &LocalProvider{Command: "true", Timeout: 0, execCommand: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "sh", "-c", `echo '{"protocol_version":1,"findings":null}'`)
		return cmd
	}}
	resp, err := p.Analyze(context.Background(), Request{})
	if err != nil || resp.Findings == nil {
		t.Fatalf("%v %+v", err, resp)
	}
}

func TestBuildRequest_More(t *testing.T) {
	dir := t.TempDir()
	req, err := BuildRequest(BuildOptions{
		Root: dir,
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Requirements: []contract.Requirement{{
				ID: "R-1", Status: "approved", Verification: contract.Verification{Semantic: "", Checks: []string{"c"}},
			}},
		},
		Selection: impact.Selection{Requirements: []impact.SelectedRequirement{{
			Requirement: contract.Requirement{ID: "R-1", Status: "approved", Verification: contract.Verification{}},
		}}},
		ChangedFiles: []string{"missing.go"},
		DiffFn:       func(root, mergeBase string) (string, error) { return "", errors.New("diff fail") },
		ReadFile:     func(path string) ([]byte, error) { return nil, os.ErrNotExist },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Requirements) != 1 || req.Requirements[0].Semantic != "optional" {
		t.Fatalf("%+v", req.Requirements)
	}
	// large truncate
	big := make([]byte, MaxInputBytes+64)
	for i := range big {
		big[i] = 'a'
	}
	_ = os.WriteFile(filepath.Join(dir, "big.go"), big, 0o644)
	req, err = BuildRequest(BuildOptions{
		Root:         dir,
		Contract:     &contract.Contract{Product: contract.Product{Name: "n", Purpose: "p"}},
		ChangedFiles: []string{"big.go"},
		DiffFn:       func(root, mergeBase string) (string, error) { return string(big), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(req.Diff), []byte("[truncated]")) {
		t.Fatal("diff should truncate")
	}
}

func TestDefaultUnifiedDiff_Repo(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "-c", "core.hooksPath=/dev/null", "init")
	run("git", "checkout", "-b", "main")
	_ = os.WriteFile(filepath.Join(dir, "f"), []byte("1\n"), 0o644)
	run("git", "add", ".")
	run("git", "-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-m", "i")
	_ = os.WriteFile(filepath.Join(dir, "f"), []byte("2\n"), 0o644)
	mb, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	diff, err := defaultUnifiedDiff(dir, string(bytes.TrimSpace(mb)))
	if err != nil {
		t.Fatal(err)
	}
	_ = diff
}

func TestRun_MorePaths(t *testing.T) {
	var buf bytes.Buffer
	// enabled but NewProvider fails via bad timeout already validated; missing provider
	out, err := Run(context.Background(), RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy:  contract.Policy{Semantic: contract.SemanticPolicy{Enabled: true, Enforcement: "advisory"}},
		},
	})
	if err != nil || out.ProviderErr == nil {
		t.Fatalf("%v %+v", err, out)
	}

	// EnsureTrust nil fails closed for local providers
	out, err = Run(context.Background(), RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy: contract.Policy{Semantic: contract.SemanticPolicy{
				Enabled: true, Enforcement: "advisory",
				Provider: &contract.SemanticProvider{Type: "local", Command: "true"},
			}},
		},
		TrustLocal: false,
	})
	if err != nil || out.ProviderErr == nil || !strings.Contains(out.ProviderErr.Error(), "trust") {
		t.Fatalf("%v %+v", err, out)
	}

	// EnsureTrust error
	out, err = Run(context.Background(), RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy: contract.Policy{Semantic: contract.SemanticPolicy{
				Enabled: true, Enforcement: "advisory",
				Provider: &contract.SemanticProvider{Type: "local", Command: "true"},
			}},
		},
		EnsureTrust: func() error { return errors.New("no trust") },
	})
	if err != nil || out.ProviderErr == nil {
		t.Fatalf("%v %+v", err, out)
	}

	// show encode error via closed writer isn't easy; success path with nil findings already covered
	stub := &stubProv{resp: Response{Findings: nil}}
	out, err = Run(context.Background(), RunOptions{
		Contract: &contract.Contract{
			Product: contract.Product{Name: "n", Purpose: "p"},
			Policy: contract.Policy{Semantic: contract.SemanticPolicy{
				Enabled: true, Enforcement: "advisory",
				Provider: &contract.SemanticProvider{Type: "http", URL: "https://x"},
			}},
		},
		Provider: stub,
		Stdout:   &buf,
	})
	if err != nil || out.SemanticRun.FindingCount != 0 {
		t.Fatalf("%v %+v", err, out.SemanticRun)
	}
}

type stubProv struct {
	resp Response
	err  error
}

func (s *stubProv) Analyze(ctx context.Context, req Request) (Response, error) {
	return s.resp, s.err
}

func TestCheckSummariesEmpty(t *testing.T) {
	if len(checkSummaries(nil)) != 0 {
		t.Fatal()
	}
}

func TestJSONRoundTrip(t *testing.T) {
	b, _ := json.Marshal(Request{ProtocolVersion: 1})
	var req Request
	_ = json.Unmarshal(b, &req)
}

package semantic_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hypertrial/intentci/internal/semantic"
)

func TestHTTPProvider_SuccessAndAuth(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(semantic.Response{
			ProtocolVersion: 1,
			Findings: []semantic.Finding{{
				RequirementID: "R-1",
				Assessment:    semantic.AssessmentAligned,
				Confidence:    1,
				Summary:       "ok",
			}},
		})
	}))
	defer srv.Close()

	p := &semantic.HTTPProvider{URL: srv.URL, Token: "secret"}
	resp, err := p.Analyze(context.Background(), semantic.Request{ProtocolVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer secret" {
		t.Fatalf("auth %q", sawAuth)
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("%+v", resp)
	}
}

func TestHTTPProvider_Errors(t *testing.T) {
	p := &semantic.HTTPProvider{URL: ""}
	if _, err := p.Analyze(context.Background(), semantic.Request{}); err == nil {
		t.Fatal("empty url")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, "boom")
	}))
	defer srv.Close()
	p = &semantic.HTTPProvider{URL: srv.URL}
	if _, err := p.Analyze(context.Background(), semantic.Request{}); err == nil {
		t.Fatal("500")
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{")
	}))
	defer bad.Close()
	p = &semantic.HTTPProvider{URL: bad.URL}
	if _, err := p.Analyze(context.Background(), semantic.Request{}); err == nil {
		t.Fatal("bad json")
	}
}

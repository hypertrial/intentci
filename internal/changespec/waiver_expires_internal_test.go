package changespec

import (
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
)

func TestValidate_WaiverExpiresInvalid(t *testing.T) {
	c := &contract.Contract{
		Version: 1,
		Product: contract.Product{Name: "x", Purpose: "y"},
		Requirements: []contract.Requirement{{
			ID: "BUILD-001", Type: "reliability", Title: "t", Statement: "s",
			Status: "approved", Severity: "blocking",
			Verification: contract.Verification{Checks: []string{"go-test"}},
		}},
		Checks: []contract.Check{{ID: "go-test", Command: "true", Timeout: "1m"}},
	}
	s := &Spec{
		Version: 1, ID: "DEMO-1", Status: "draft", Type: "feature", Summary: "s",
		Goals: []string{"g"},
		Acceptance: []Acceptance{{
			ID: "AC-001", Statement: "s", Severity: "blocking",
			Verification: Verification{Checks: []string{"go-test"}},
		}},
		Waivers: []Waiver{{
			ID: "W-001", Requirement: "BUILD-001", Reason: "r", Owner: "a", Expires: "2026-13-40",
		}},
	}
	if err := Validate(s, c); err == nil {
		t.Fatal("bad date")
	}
	old := nowUTC
	defer func() { nowUTC = old }()
	nowUTC = func() time.Time { return time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC) }
	s.Waivers[0].Expires = "2026-07-25"
	if err := Validate(s, c); err != nil {
		t.Fatal(err)
	}
}

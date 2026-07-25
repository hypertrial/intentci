package changespec_test

import (
	"testing"
	"time"

	"github.com/hypertrial/intentci/internal/changespec"
)

func TestValidate_Waivers(t *testing.T) {
	c := validContract()
	s := &changespec.Spec{
		Version: 1, ID: "DEMO-1", Status: "draft", Type: "feature", Summary: "s",
		Goals: []string{"g"},
		Acceptance: []changespec.Acceptance{{
			ID: "AC-001", Statement: "s", Severity: "blocking",
			Verification: changespec.Verification{Checks: []string{"go-test"}},
		}},
		Waivers: []changespec.Waiver{{
			ID: "W-001", Requirement: "BUILD-001", Reason: "temp", Owner: "alice",
			Expires: time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02"),
		}},
	}
	if err := changespec.Validate(s, c); err != nil {
		t.Fatal(err)
	}
	s.Waivers[0].Expires = "2000-01-01"
	if err := changespec.Validate(s, c); err == nil {
		t.Fatal("expired")
	}
	s.Waivers[0].Expires = time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	s.Waivers[0].Owner = "alice"
	s.Waivers[0].Requirement = "NOPE"
	if err := changespec.Validate(s, c); err == nil {
		t.Fatal("unknown req")
	}
	s.Waivers = []changespec.Waiver{
		{ID: "W-001", Requirement: "AC-001", Reason: "r", Owner: "a", Expires: time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")},
		{ID: "W-001", Requirement: "AC-001", Reason: "r", Owner: "a", Expires: time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")},
	}
	if err := changespec.Validate(s, c); err == nil {
		t.Fatal("duplicate waiver id")
	}
}

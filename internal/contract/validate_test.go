package contract_test

import (
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/contract"
)

func validContract() *contract.Contract {
	unknown := true
	unverified := true
	return &contract.Contract{
		Version: 1,
		Product: contract.Product{
			Name:    "demo",
			Purpose: "Demo product",
		},
		Policy: contract.Policy{
			DefaultBase:      "origin/main",
			UnknownBlocks:    &unknown,
			UnverifiedBlocks: &unverified,
		},
		Requirements: []contract.Requirement{
			{
				ID:        "STATE-001",
				Type:      "invariant",
				Title:     "State preserved",
				Statement: "Failed writes must not advance state.",
				Status:    "approved",
				Severity:  "blocking",
				AppliesTo: contract.AppliesTo{
					Include: []string{"src/state/**"},
				},
				Verification: contract.Verification{
					Mode:   "all",
					Checks: []string{"state-unit"},
				},
			},
		},
		Checks: []contract.Check{
			{
				ID:       "state-unit",
				Command:  "go test ./...",
				Profiles: []string{"fast", "full"},
				Inputs:   []string{"src/state/**"},
				Timeout:  "5m",
			},
		},
	}
}

func TestValidate_OK(t *testing.T) {
	if err := contract.Validate(validContract()); err != nil {
		t.Fatalf("expected valid contract: %v", err)
	}
}

func TestValidate_DuplicateRequirementID(t *testing.T) {
	c := validContract()
	c.Requirements = append(c.Requirements, c.Requirements[0])
	err := contract.Validate(c)
	if err == nil || !strings.Contains(err.Error(), "duplicate requirement") {
		t.Fatalf("expected duplicate requirement error, got %v", err)
	}
}

func TestValidate_UnknownCheckRef(t *testing.T) {
	c := validContract()
	c.Requirements[0].Verification.Checks = []string{"missing-check"}
	err := contract.Validate(c)
	if err == nil || !strings.Contains(err.Error(), "unknown check") {
		t.Fatalf("expected unknown check error, got %v", err)
	}
}

func TestValidate_DependencyCycle(t *testing.T) {
	c := validContract()
	c.Checks = []contract.Check{
		{ID: "a", Command: "true", DependsOn: []string{"b"}, Timeout: "1m"},
		{ID: "b", Command: "true", DependsOn: []string{"a"}, Timeout: "1m"},
	}
	c.Requirements[0].Verification.Checks = []string{"a"}
	err := contract.Validate(c)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestValidate_InvalidGlob(t *testing.T) {
	c := validContract()
	c.Requirements[0].AppliesTo.Include = []string{"[invalid"}
	err := contract.Validate(c)
	if err == nil || !strings.Contains(err.Error(), "invalid glob") {
		t.Fatalf("expected invalid glob error, got %v", err)
	}
}

func TestApprovedBlocking_IgnoresDraft(t *testing.T) {
	c := validContract()
	c.Requirements = append(c.Requirements, contract.Requirement{
		ID:        "DRAFT-001",
		Type:      "behavior",
		Title:     "Draft",
		Statement: "Draft requirement",
		Status:    "draft",
		Severity:  "blocking",
		Verification: contract.Verification{
			Checks: []string{"state-unit"},
		},
	})
	got := c.ApprovedBlocking()
	if len(got) != 1 || got[0].ID != "STATE-001" {
		t.Fatalf("expected only STATE-001, got %#v", got)
	}
}

package changespec

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/hypertrial/intentci/pkg/protocol"
)

// DiffApproved compares base and head Change Spec bytes for approved-spec integrity.
// Draft-only work with no approved base produces no findings. Demoting or editing an
// approved base always emits a finding.
func DiffApproved(id string, baseData []byte, baseOK bool, head *Spec, headData []byte) []protocol.ChangeFinding {
	if !baseOK {
		if head.Status != "approved" {
			return nil
		}
		return []protocol.ChangeFinding{{
			Type:    "change_spec_added",
			Summary: fmt.Sprintf("Approved Change Spec %s was added in this change.", id),
		}}
	}
	if bytes.Equal(baseData, headData) {
		return nil
	}
	var base Spec
	_ = yaml.Unmarshal(baseData, &base)
	if head.Status != "approved" {
		if base.Status == "approved" {
			return []protocol.ChangeFinding{{
				Type:    "change_spec_modified",
				Summary: fmt.Sprintf("Approved Change Spec %s was modified relative to the base commit.", id),
			}}
		}
		return nil
	}
	if base.Status == "approved" {
		return []protocol.ChangeFinding{{
			Type:    "change_spec_modified",
			Summary: fmt.Sprintf("Approved Change Spec %s was modified relative to the base commit.", id),
		}}
	}
	return []protocol.ChangeFinding{{
		Type:    "change_spec_approved",
		Summary: fmt.Sprintf("Change Spec %s became approved in this change.", id),
	}}
}

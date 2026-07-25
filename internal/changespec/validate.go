package changespec

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/hypertrial/intentci/internal/contract"
	appschema "github.com/hypertrial/intentci/pkg/schema"
)

// nowUTC is overridable for expiry tests.
var nowUTC = func() time.Time { return time.Now().UTC() }

// ValidationError collects Change Spec problems.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return "invalid Change Spec:\n  - " + strings.Join(e.Errors, "\n  - ")
}

func (e *ValidationError) add(format string, args ...any) {
	e.Errors = append(e.Errors, fmt.Sprintf(format, args...))
}

// Validate validates a Change Spec against schema and a Product Contract.
func Validate(s *Spec, c *contract.Contract) error {
	ve := &ValidationError{}
	if err := validateSchema(s); err != nil {
		ve.add("%s", err.Error())
		return ve
	}
	seenAC := map[string]int{}
	checks := c.CheckMap()
	for i, ac := range s.Acceptance {
		if prev, ok := seenAC[ac.ID]; ok {
			ve.add("duplicate acceptance id %q (indexes %d and %d)", ac.ID, prev, i)
		} else {
			seenAC[ac.ID] = i
		}
		for _, id := range ac.Verification.Checks {
			if _, ok := checks[id]; !ok {
				ve.add("acceptance %q references unknown check %q", ac.ID, id)
			}
		}
	}
	for _, id := range s.RequiredChecks {
		if _, ok := checks[id]; !ok {
			ve.add("required_checks references unknown check %q", id)
		}
	}
	// Force-selection only applies to approved+blocking requirements (impact.Resolve).
	forceable := map[string]struct{}{}
	for _, r := range c.ApprovedBlocking() {
		forceable[r.ID] = struct{}{}
	}
	for _, id := range s.AffectedRequirements {
		if _, ok := forceable[id]; !ok {
			ve.add("affected_requirements references requirement %q which is not approved and blocking", id)
		}
	}
	acIDs := map[string]struct{}{}
	for _, ac := range s.Acceptance {
		acIDs[ac.ID] = struct{}{}
	}
	seenWaiver := map[string]int{}
	for i, w := range s.Waivers {
		if prev, ok := seenWaiver[w.ID]; ok {
			ve.add("duplicate waiver id %q (indexes %d and %d)", w.ID, prev, i)
		} else {
			seenWaiver[w.ID] = i
		}
		if _, ok := forceable[w.Requirement]; !ok {
			if _, acOK := acIDs[w.Requirement]; !acOK {
				ve.add("waiver %q references unknown requirement %q", w.ID, w.Requirement)
			}
		}
		exp, err := time.Parse("2006-01-02", w.Expires)
		if err != nil {
			ve.add("waiver %q has invalid expires date %q", w.ID, w.Expires)
			continue
		}
		today := nowUTC()
		today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		if exp.Before(today) {
			ve.add("waiver %q expired on %s", w.ID, w.Expires)
		}
	}
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

var (
	schemaJSON = func() []byte { return appschema.ChangeSpecJSON }
	addSchemaResource = func(c *jsonschema.Compiler, url string, doc any) error {
		return c.AddResource(url, doc)
	}
	compileSchemaURL = func(c *jsonschema.Compiler, url string) (*jsonschema.Schema, error) {
		return c.Compile(url)
	}
)

func validateSchema(s *Spec) error {
	sch, err := compileSchema(schemaJSON(), "https://intentci.dev/schemas/changespec-v1.json")
	if err != nil {
		return err
	}
	m := ToJSONMap(s)
	normalize(m)
	if err := sch.Validate(m); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}

func compileSchema(raw []byte, url string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	if err := addSchemaResource(compiler, url, doc); err != nil {
		return nil, err
	}
	return compileSchemaURL(compiler, url)
}

func normalize(m map[string]any) {
	stripNulls(m)
	if src, ok := m["source"].(map[string]any); ok && len(src) == 0 {
		delete(m, "source")
	}
	if ng, ok := m["non_goals"].([]any); ok && len(ng) == 0 {
		delete(m, "non_goals")
	}
	if ar, ok := m["affected_requirements"].([]any); ok && len(ar) == 0 {
		delete(m, "affected_requirements")
	}
	if rc, ok := m["required_checks"].([]any); ok && len(rc) == 0 {
		delete(m, "required_checks")
	}
	if w, ok := m["waivers"].([]any); ok {
		if len(w) == 0 {
			delete(m, "waivers")
		} else {
			for _, item := range w {
				wm, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if s, ok := wm["owner"].(string); ok && s == "" {
					delete(wm, "owner")
				}
				if s, ok := wm["approver"].(string); ok && s == "" {
					delete(wm, "approver")
				}
			}
		}
	}
	if acc, ok := m["acceptance"].([]any); ok {
		for _, a := range acc {
			am, ok := a.(map[string]any)
			if !ok {
				continue
			}
			if ver, ok := am["verification"].(map[string]any); ok {
				if sem, ok := ver["semantic"].(string); ok && sem == "" {
					delete(ver, "semantic")
				}
			}
		}
	}
}

func stripNulls(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if val == nil {
				delete(t, k)
				continue
			}
			stripNulls(val)
		}
	case []any:
		for _, val := range t {
			stripNulls(val)
		}
	}
}

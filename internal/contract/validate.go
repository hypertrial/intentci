package contract

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/santhosh-tekuri/jsonschema/v6"

	appschema "github.com/hypertrial/intentci/pkg/schema"
)

// ValidationError collects contract validation problems.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return "invalid Product Contract:\n  - " + strings.Join(e.Errors, "\n  - ")
}

func (e *ValidationError) add(format string, args ...any) {
	e.Errors = append(e.Errors, fmt.Sprintf(format, args...))
}

func (e *ValidationError) empty() bool {
	return len(e.Errors) == 0
}

// Validate performs schema and semantic validation.
func Validate(c *Contract) error {
	ve := &ValidationError{}

	if err := validateSchema(c); err != nil {
		ve.add("%s", err.Error())
		return ve
	}

	validateDuplicates(c, ve)
	validateCheckRefs(c, ve)
	validateDependencyCycles(c, ve)
	validateGlobs(c, ve)
	validateTimeouts(c, ve)

	if !ve.empty() {
		return ve
	}
	return nil
}

func validateSchema(c *Contract) error {
	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(appschema.ContractJSON))
	if err != nil {
		return fmt.Errorf("parse embedded contract schema: %w", err)
	}
	if err := compiler.AddResource("https://intentci.dev/schemas/contract-v1.json", doc); err != nil {
		return fmt.Errorf("add contract schema: %w", err)
	}
	sch, err := compiler.Compile("https://intentci.dev/schemas/contract-v1.json")
	if err != nil {
		return fmt.Errorf("compile contract schema: %w", err)
	}
	m, err := ToJSONMap(c)
	if err != nil {
		return fmt.Errorf("convert contract for schema validation: %w", err)
	}
	// Remove empty optional nested objects that YAML may omit as zero values
	// but JSON marshal includes as empty objects violating additionalProperties.
	normalizeForSchema(m)
	if err := sch.Validate(m); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	return nil
}

func normalizeForSchema(m map[string]any) {
	stripNulls(m)
	if policy, ok := m["policy"].(map[string]any); ok {
		if semantic, ok := policy["semantic"].(map[string]any); ok {
			enabled, hasEnabled := semantic["enabled"].(bool)
			enforcement, _ := semantic["enforcement"].(string)
			if (!hasEnabled || !enabled) && enforcement == "" {
				delete(policy, "semantic")
			}
		}
		if len(policy) == 0 {
			delete(m, "policy")
		}
	}
	if exec, ok := m["execution"].(map[string]any); ok {
		if mp, ok := exec["max_parallel"].(float64); ok && mp < 1 {
			delete(exec, "max_parallel")
		}
		if len(exec) == 0 {
			delete(m, "execution")
		}
	}
	if env, ok := m["environment"].(map[string]any); ok && len(env) == 0 {
		delete(m, "environment")
	}
	if reqs, ok := m["requirements"].([]any); ok {
		for _, r := range reqs {
			rm, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if applies, ok := rm["applies_to"].(map[string]any); ok && len(applies) == 0 {
				delete(rm, "applies_to")
			}
			if sources, ok := rm["sources"].([]any); ok && len(sources) == 0 {
				delete(rm, "sources")
			}
			if ver, ok := rm["verification"].(map[string]any); ok {
				if mode, ok := ver["mode"].(string); ok && mode == "" {
					delete(ver, "mode")
				}
				if sem, ok := ver["semantic"].(string); ok && sem == "" {
					delete(ver, "semantic")
				}
			}
		}
	}
	if checks, ok := m["checks"].([]any); ok {
		for _, ch := range checks {
			cm, ok := ch.(map[string]any)
			if !ok {
				continue
			}
			if profiles, ok := cm["profiles"].([]any); ok && len(profiles) == 0 {
				delete(cm, "profiles")
			}
			if inputs, ok := cm["inputs"].([]any); ok && len(inputs) == 0 {
				delete(cm, "inputs")
			}
			if deps, ok := cm["depends_on"].([]any); ok && len(deps) == 0 {
				delete(cm, "depends_on")
			}
			if timeout, ok := cm["timeout"].(string); ok && timeout == "" {
				delete(cm, "timeout")
			}
			if cache, ok := cm["cache"].(string); ok && cache == "" {
				delete(cm, "cache")
			}
			if results, ok := cm["results"].(map[string]any); ok {
				if len(results) == 0 || (results["format"] == "" && results["path"] == "") {
					delete(cm, "results")
				}
			}
			if exclusive, ok := cm["exclusive"].(bool); ok && !exclusive {
				delete(cm, "exclusive")
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

func validateDuplicates(c *Contract, ve *ValidationError) {
	seenReq := map[string]int{}
	for i, r := range c.Requirements {
		if prev, ok := seenReq[r.ID]; ok {
			ve.add("duplicate requirement id %q (indexes %d and %d)", r.ID, prev, i)
		} else {
			seenReq[r.ID] = i
		}
	}
	seenCheck := map[string]int{}
	for i, ch := range c.Checks {
		if prev, ok := seenCheck[ch.ID]; ok {
			ve.add("duplicate check id %q (indexes %d and %d)", ch.ID, prev, i)
		} else {
			seenCheck[ch.ID] = i
		}
	}
}

func validateCheckRefs(c *Contract, ve *ValidationError) {
	checks := c.CheckMap()
	for _, r := range c.Requirements {
		for _, id := range r.Verification.Checks {
			if _, ok := checks[id]; !ok {
				ve.add("requirement %q references unknown check %q", r.ID, id)
			}
		}
	}
	for _, ch := range c.Checks {
		for _, dep := range ch.DependsOn {
			if _, ok := checks[dep]; !ok {
				ve.add("check %q depends on unknown check %q", ch.ID, dep)
			}
		}
	}
}

func validateDependencyCycles(c *Contract, ve *ValidationError) {
	deps := map[string][]string{}
	for _, ch := range c.Checks {
		deps[ch.ID] = append([]string{}, ch.DependsOn...)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var path []string
	var visit func(string)
	visit = func(id string) {
		color[id] = gray
		path = append(path, id)
		for _, d := range deps[id] {
			switch color[d] {
			case gray:
				ve.add("check dependency cycle involving %s", strings.Join(append(path, d), " -> "))
			case white:
				visit(d)
			}
		}
		path = path[:len(path)-1]
		color[id] = black
	}
	for _, ch := range c.Checks {
		if color[ch.ID] == white {
			visit(ch.ID)
		}
	}
}

func validateGlobs(c *Contract, ve *ValidationError) {
	checkGlob := func(owner, field, pattern string) {
		if pattern == "" {
			ve.add("%s %s: empty glob", owner, field)
			return
		}
		if !doublestar.ValidatePattern(pattern) {
			ve.add("%s %s: invalid glob %q", owner, field, pattern)
		}
	}
	for _, r := range c.Requirements {
		for _, g := range r.AppliesTo.Include {
			checkGlob("requirement "+r.ID, "applies_to.include", g)
		}
		for _, g := range r.AppliesTo.Exclude {
			checkGlob("requirement "+r.ID, "applies_to.exclude", g)
		}
	}
	for _, ch := range c.Checks {
		for _, g := range ch.Inputs {
			checkGlob("check "+ch.ID, "inputs", g)
		}
	}
}

func validateTimeouts(c *Contract, ve *ValidationError) {
	for _, ch := range c.Checks {
		if ch.Timeout == "" {
			continue
		}
		if _, err := ParseTimeout(ch.Timeout); err != nil {
			ve.add("check %q has invalid timeout %q: %v", ch.ID, ch.Timeout, err)
		}
	}
}

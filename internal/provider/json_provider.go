package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/security"
)

// JSONProvider reads a JSON file and checks assert equality on a key path.
type JSONProvider struct{}

func (p *JSONProvider) Name() string    { return "json" }
func (p *JSONProvider) Version() string { return "1.0.0" }

func (p *JSONProvider) Validate(spec ir.ProviderSpec) []Diagnostic {
	if spec.Report == "" {
		return []Diagnostic{{Message: "report required"}}
	}
	if len(spec.Assert) == 0 {
		return []Diagnostic{{Message: "assert required"}}
	}
	if err := validateJSONAssert(spec.Assert); err != nil {
		return []Diagnostic{{Message: err.Error()}}
	}
	return nil
}

func (p *JSONProvider) Execute(ctx context.Context, req Request) Result {
	_ = ctx
	start := time.Now()
	report, err := security.ResolveInside(req.Root, req.Spec.Report)
	if err != nil {
		return jsonError(p, req, start, err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return jsonError(p, req, start, err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return Result{
			Provider: p.Name(), ProviderVersion: p.Version(), Status: "error",
			DurationMS:  time.Since(start).Milliseconds(),
			Diagnostics: []string{err.Error()},
		}
	}
	passed := true
	summary := "json assert passed"
	if req.Spec.Assert != nil {
		var err error
		passed, summary, err = evaluateJSONAssert(doc, req.Spec.Assert)
		if err != nil {
			return jsonError(p, req, start, err)
		}
	}
	return Result{
		Provider: p.Name(), ProviderVersion: p.Version(), Status: "completed",
		DurationMS: time.Since(start).Milliseconds(),
		Evidence: []Evidence{{
			ID: firstNonEmpty(req.Spec.ID, "json"), Class: firstNonEmpty(req.Spec.EvidenceClass, req.EvidenceClass, "deterministic"),
			Summary: summary, Passed: boolPtr(passed),
		}},
	}
}

func jsonError(p *JSONProvider, req Request, start time.Time, err error) Result {
	return Result{
		Provider: p.Name(), ProviderVersion: p.Version(), Status: "error",
		DurationMS: time.Since(start).Milliseconds(), Diagnostics: []string{err.Error()},
		SecurityViolation: security.IsPathViolation(err),
		Evidence:          []Evidence{{ID: req.Spec.ID, Class: "deterministic", Summary: err.Error(), Passed: boolPtr(false)}},
	}
}

func validateJSONAssert(assert map[string]any) error {
	if assert == nil {
		return nil
	}
	path, hasPath := assert["path"].(string)
	if !hasPath {
		return nil // legacy top-level equality map
	}
	if _, _, err := jsonPath(nil, path, true); err != nil {
		return err
	}
	operators := 0
	for _, name := range []string{"exists", "equals", "not_equals", "gt", "gte", "lt", "lte"} {
		if _, ok := assert[name]; ok {
			operators++
		}
	}
	if operators != 1 {
		return fmt.Errorf("json assert requires exactly one supported operator")
	}
	return nil
}

func evaluateJSONAssert(doc any, assert map[string]any) (bool, string, error) {
	if err := validateJSONAssert(assert); err != nil {
		return false, "", err
	}
	path, pathMode := assert["path"].(string)
	if !pathMode {
		keys := make([]string, 0, len(assert))
		for key := range assert {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			got, exists := lookupMember(doc, key)
			if !exists || fmt.Sprint(got) != fmt.Sprint(assert[key]) {
				return false, fmt.Sprintf("assert %s: got %v want %v", key, got, assert[key]), nil
			}
		}
		return true, "json assert passed", nil
	}
	got, exists, _ := jsonPath(doc, path, false)
	if want, ok := assert["exists"]; ok {
		expected, ok := want.(bool)
		if !ok {
			return false, "", fmt.Errorf("exists must be boolean")
		}
		return exists == expected, fmt.Sprintf("%s exists=%t", path, exists), nil
	}
	if !exists {
		return false, path + " does not exist", nil
	}
	operator := ""
	for _, candidate := range []string{"equals", "not_equals", "gt", "gte", "lt", "lte"} {
		if _, ok := assert[candidate]; ok {
			operator = candidate
			break
		}
	}
	want := assert[operator]
	passed, err := compareJSON(got, want, operator)
	return passed, fmt.Sprintf("%s %s %v (got %v)", path, operator, want, got), err
}

func jsonPath(doc any, expression string, validateOnly bool) (any, bool, error) {
	if expression == "" || expression[0] != '$' {
		return nil, false, fmt.Errorf("unsupported JSONPath %q", expression)
	}
	current := doc
	exists := true
	for i := 1; i < len(expression); {
		switch expression[i] {
		case '.':
			i++
			start := i
			for i < len(expression) && (expression[i] == '_' || expression[i] == '-' ||
				expression[i] >= 'a' && expression[i] <= 'z' ||
				expression[i] >= 'A' && expression[i] <= 'Z' ||
				expression[i] >= '0' && expression[i] <= '9') {
				i++
			}
			if start == i {
				return nil, false, fmt.Errorf("unsupported JSONPath %q", expression)
			}
			if !validateOnly && exists {
				current, exists = lookupMember(current, expression[start:i])
			}
		case '[':
			end := strings.IndexByte(expression[i:], ']')
			if end < 0 {
				return nil, false, fmt.Errorf("unsupported JSONPath %q", expression)
			}
			end += i
			index, err := strconv.Atoi(expression[i+1 : end])
			if err != nil || index < 0 {
				return nil, false, fmt.Errorf("unsupported JSONPath %q", expression)
			}
			if !validateOnly && exists {
				array, ok := current.([]any)
				if !ok || index >= len(array) {
					exists = false
				} else {
					current = array[index]
				}
			}
			i = end + 1
		default:
			return nil, false, fmt.Errorf("unsupported JSONPath %q", expression)
		}
	}
	return current, exists, nil
}

func lookupMember(doc any, key string) (any, bool) {
	m, ok := doc.(map[string]any)
	if !ok {
		return nil, false
	}
	value, exists := m[key]
	return value, exists
}

func compareJSON(got, want any, operator string) (bool, error) {
	if operator == "equals" {
		return fmt.Sprint(got) == fmt.Sprint(want), nil
	}
	if operator == "not_equals" {
		return fmt.Sprint(got) != fmt.Sprint(want), nil
	}
	left, leftOK := number(got)
	right, rightOK := number(want)
	if !leftOK || !rightOK {
		return false, fmt.Errorf("%s requires numeric operands", operator)
	}
	switch operator {
	case "gt":
		return left > right, nil
	case "gte":
		return left >= right, nil
	case "lt":
		return left < right, nil
	case "lte":
		return left <= right, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", operator)
	}
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

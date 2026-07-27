package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hypertrial/intentci/internal/ir"
)

var idRE = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{1,31}-[0-9]{1,8}$`)
var yamlLineRE = regexp.MustCompile(`line ([0-9]+)`)

// Diagnostic is a parse/compile diagnostic.
type Diagnostic struct {
	Severity string `json:"severity,omitempty"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
}

func (d Diagnostic) Error() string {
	location := d.Path
	if d.Line > 0 {
		location += fmt.Sprintf(":%d", d.Line)
		if d.Column > 0 {
			location += fmt.Sprintf(":%d", d.Column)
		}
	}
	if location == "" {
		return d.Message
	}
	return location + ": " + d.Message
}

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// ParseFile parses a single Markdown requirement file.
func ParseFile(path string) (ir.Requirement, []Diagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ir.Requirement{}, []Diagnostic{{Path: path, Message: err.Error()}}
	}
	return Parse(path, data)
}

// Parse parses Markdown with YAML front matter.
func Parse(path string, data []byte) (ir.Requirement, []Diagnostic) {
	var diags []Diagnostic
	text := string(data)
	fm, body, err := splitFrontMatter(text)
	if err != nil {
		return ir.Requirement{}, locateDiagnostics(text, []Diagnostic{{Path: path, Message: err.Error()}})
	}

	var meta struct {
		ID        string   `yaml:"id"`
		Title     string   `yaml:"title"`
		Status    string   `yaml:"status"`
		Priority  string   `yaml:"priority"`
		Owners    []string `yaml:"owners"`
		DependsOn []string `yaml:"depends_on"`
		AppliesTo struct {
			Paths   []string `yaml:"paths"`
			Symbols []string `yaml:"symbols"`
		} `yaml:"applies_to"`
		Tags    []string `yaml:"tags"`
		Timeout string   `yaml:"timeout"`
	}
	if err := decodeKnown(fm, &meta); err != nil {
		return ir.Requirement{}, locateDiagnostics(text, []Diagnostic{{Path: path, Message: "front matter: " + err.Error()}})
	}

	req := ir.Requirement{
		ID:         meta.ID,
		Title:      meta.Title,
		Status:     meta.Status,
		Priority:   meta.Priority,
		Owners:     meta.Owners,
		DependsOn:  meta.DependsOn,
		AppliesTo:  ir.AppliesTo{Paths: meta.AppliesTo.Paths, Symbols: meta.AppliesTo.Symbols},
		Tags:       meta.Tags,
		Timeout:    meta.Timeout,
		SourcePath: filepath.ToSlash(path),
	}

	if req.ID == "" {
		diags = append(diags, Diagnostic{Path: path, Message: "missing id"})
	} else if !idRE.MatchString(req.ID) {
		diags = append(diags, Diagnostic{Path: path, Message: "invalid id format"})
	}
	if req.Title == "" {
		diags = append(diags, Diagnostic{Path: path, Message: "missing title"})
	}
	if req.Status == "" {
		diags = append(diags, Diagnostic{Path: path, Message: "missing status"})
	} else if !oneOf(req.Status, "draft", "active", "deprecated", "superseded", "disabled") {
		diags = append(diags, Diagnostic{Path: path, Message: "invalid status " + fmt.Sprintf("%q", req.Status)})
	}
	if req.Priority == "" {
		diags = append(diags, Diagnostic{Path: path, Message: "missing priority"})
	} else if !oneOf(req.Priority, "required", "recommended", "informational") {
		diags = append(diags, Diagnostic{Path: path, Message: "invalid priority " + fmt.Sprintf("%q", req.Priority)})
	}

	sections := splitSections(body)
	req.Intent = strings.TrimSpace(sections["intent"])
	req.Rationale = strings.TrimSpace(sections["rationale"])
	if req.Intent == "" {
		diags = append(diags, Diagnostic{Path: path, Message: "missing # Intent section"})
	}

	if raw, ok := sections["constraints"]; ok {
		cons, d := parseConstraints(path, raw)
		req.Constraints = cons
		diags = append(diags, d...)
	}
	if raw, ok := sections["boundaries"]; ok {
		b, d := parseBoundaries(path, raw)
		req.Boundaries = b
		diags = append(diags, d...)
	}
	if raw, ok := sections["obligations"]; ok {
		obs, d := parseObligations(path, raw)
		req.Obligations = obs
		diags = append(diags, d...)
	}
	if len(req.Obligations) == 0 {
		diags = append(diags, Diagnostic{Path: path, Message: "at least one obligation is required"})
	}
	return req, locateDiagnostics(text, diags)
}

func locateDiagnostics(source string, diagnostics []Diagnostic) []Diagnostic {
	lines := strings.Split(source, "\n")
	for index := range diagnostics {
		diagnostic := &diagnostics[index]
		if diagnostic.Line > 0 {
			continue
		}
		if match := yamlLineRE.FindStringSubmatch(diagnostic.Message); len(match) == 2 {
			fmt.Sscanf(match[1], "%d", &diagnostic.Line)
			diagnostic.Line++ // account for the opening front-matter delimiter
			diagnostic.Column = 1
			continue
		}
		token := strings.TrimSpace(strings.SplitN(diagnostic.Message, ":", 2)[0])
		for lineIndex, line := range lines {
			column := strings.Index(line, token)
			if token != "" && column >= 0 {
				diagnostic.Line = lineIndex + 1
				diagnostic.Column = column + 1
				break
			}
		}
		if diagnostic.Line == 0 {
			diagnostic.Line = 1
			diagnostic.Column = 1
		}
	}
	return diagnostics
}

func splitFrontMatter(text string) (string, string, error) {
	text = strings.TrimPrefix(text, "\uFEFF")
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return "", "", fmt.Errorf("missing YAML front matter")
	}
	rest := text[4:]
	if strings.HasPrefix(text, "---\r\n") {
		rest = text[5:]
	}
	idx := strings.Index(rest, "\n---\n")
	crIdx := strings.Index(rest, "\n---\r\n")
	end := -1
	sepLen := 5
	if idx >= 0 {
		end = idx
	}
	if crIdx >= 0 && (end < 0 || crIdx < end) {
		end = crIdx
		sepLen = 6
	}
	if end < 0 {
		// closing --- at EOF (no trailing newline)
		if i := strings.Index(rest, "\n---"); i >= 0 && rest[i+4:] == "" {
			end = i
			sepLen = 4
		}
	}
	if end < 0 {
		return "", "", fmt.Errorf("unterminated YAML front matter")
	}
	fm := rest[:end]
	body := rest[end+sepLen:]
	return fm, body, nil
}

func splitSections(body string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(body, "\n")
	var cur string
	var buf []string
	flush := func() {
		if cur != "" {
			out[cur] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
		buf = nil
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			flush()
			cur = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "# ")))
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return out
}

func parseConstraints(path, raw string) ([]ir.Constraint, []Diagnostic) {
	var diags []Diagnostic
	var out []ir.Constraint
	// Expect ## Must / ## Must Not subsections with YAML lists
	parts := splitH2(raw)
	kinds := make([]string, 0, len(parts))
	for kind := range parts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		body := parts[kind]
		var items []struct {
			ID        string `yaml:"id"`
			Statement string `yaml:"statement"`
		}
		// body may be a yaml list directly
		trimmed := strings.TrimSpace(body)
		if trimmed == "" {
			continue
		}
		if err := decodeKnown(trimmed, &items); err != nil {
			diags = append(diags, Diagnostic{Path: path, Message: "constraints: " + err.Error()})
			continue
		}
		k := "must"
		if strings.Contains(strings.ToLower(kind), "not") {
			k = "must_not"
		}
		for _, it := range items {
			out = append(out, ir.Constraint{ID: it.ID, Kind: k, Statement: it.Statement})
		}
	}
	return out, diags
}

func oneOf(got string, values ...string) bool {
	for _, value := range values {
		if got == value {
			return true
		}
	}
	return false
}

func splitH2(raw string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(raw, "\n")
	var cur string
	var buf []string
	flush := func() {
		if cur != "" {
			out[cur] = strings.Join(buf, "\n")
		}
		buf = nil
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			cur = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return out
}

func parseBoundaries(path, raw string) (ir.Boundaries, []Diagnostic) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```yaml")
	raw = strings.TrimPrefix(raw, "```yml")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var b ir.Boundaries
	if err := decodeKnown(raw, &b); err != nil {
		return b, []Diagnostic{{Path: path, Message: "boundaries: " + err.Error()}}
	}
	return b, nil
}

type obligationYAML struct {
	ID                  string         `yaml:"id"`
	Statement           string         `yaml:"statement"`
	Required            *bool          `yaml:"required"`
	Description         string         `yaml:"description"`
	Rationale           string         `yaml:"rationale"`
	EvidenceClass       string         `yaml:"evidence_class"`
	ConfidenceThreshold *float64       `yaml:"confidence_threshold"`
	Timeout             string         `yaml:"timeout"`
	Retry               ir.Retry       `yaml:"retry"`
	Platforms           []string       `yaml:"platforms"`
	Tags                []string       `yaml:"tags"`
	DependsOn           []string       `yaml:"depends_on"`
	ManualReview        bool           `yaml:"manual_review"`
	Severity            string         `yaml:"severity"`
	Verify              map[string]any `yaml:"verify"`
}

func parseObligations(path, raw string) ([]ir.Obligation, []Diagnostic) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```yaml")
	raw = strings.TrimPrefix(raw, "```yml")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var items []obligationYAML
	if err := decodeKnown(raw, &items); err != nil {
		return nil, []Diagnostic{{Path: path, Message: "obligations: " + err.Error()}}
	}
	var out []ir.Obligation
	var diags []Diagnostic
	for _, it := range items {
		if it.ID == "" {
			diags = append(diags, Diagnostic{Path: path, Message: "obligation missing id"})
			continue
		}
		if strings.TrimSpace(it.Statement) == "" {
			diags = append(diags, Diagnostic{Path: path, Message: it.ID + ": missing statement"})
		}
		req := true
		if it.Required == nil {
			diags = append(diags, Diagnostic{Path: path, Message: it.ID + ": missing required"})
		} else {
			req = *it.Required
		}
		node, err := mapToVerify(it.Verify)
		if err != nil {
			diags = append(diags, Diagnostic{Path: path, Message: it.ID + ": " + err.Error()})
			continue
		}
		out = append(out, ir.Obligation{
			ID: it.ID, Statement: it.Statement, Required: req,
			Description: it.Description, Rationale: it.Rationale,
			EvidenceClass: it.EvidenceClass, ConfidenceThreshold: it.ConfidenceThreshold,
			Timeout: it.Timeout, Retry: it.Retry, Platforms: it.Platforms, Tags: it.Tags,
			DependsOn: it.DependsOn, ManualReview: it.ManualReview, Severity: it.Severity,
			Verify: node,
		})
	}
	return out, diags
}

func mapToVerify(m map[string]any) (ir.VerifyNode, error) {
	if m == nil {
		return ir.VerifyNode{}, fmt.Errorf("missing verify")
	}
	operators := 0
	for _, key := range []string{"all", "any", "not", "provider"} {
		if _, ok := m[key]; ok {
			operators++
		}
	}
	if operators != 1 {
		return ir.VerifyNode{}, fmt.Errorf("verify must contain exactly one of all, any, not, or provider")
	}
	if v, ok := m["all"]; ok {
		nodes, err := toNodeList(v)
		if err != nil {
			return ir.VerifyNode{}, err
		}
		if len(nodes) == 0 {
			return ir.VerifyNode{}, fmt.Errorf("all must not be empty")
		}
		return ir.VerifyNode{All: nodes}, nil
	}
	if v, ok := m["any"]; ok {
		nodes, err := toNodeList(v)
		if err != nil {
			return ir.VerifyNode{}, err
		}
		if len(nodes) == 0 {
			return ir.VerifyNode{}, fmt.Errorf("any must not be empty")
		}
		return ir.VerifyNode{Any: nodes}, nil
	}
	if v, ok := m["not"]; ok {
		childMap, ok := v.(map[string]any)
		if !ok {
			return ir.VerifyNode{}, fmt.Errorf("not must be a mapping")
		}
		child, err := mapToVerify(childMap)
		if err != nil {
			return ir.VerifyNode{}, err
		}
		return ir.VerifyNode{Not: &child}, nil
	}
	spec, err := toProvider(m)
	if err != nil {
		return ir.VerifyNode{}, err
	}
	return ir.VerifyNode{Provider: &spec}, nil
}

func toNodeList(v any) ([]ir.VerifyNode, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected list")
	}
	out := make([]ir.VerifyNode, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("verify item must be a mapping")
		}
		// leaf provider or nested
		if _, has := m["provider"]; has {
			spec, err := toProvider(m)
			if err != nil {
				return nil, err
			}
			out = append(out, ir.VerifyNode{Provider: &spec})
			continue
		}
		n, err := mapToVerify(m)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func toProvider(m map[string]any) (ir.ProviderSpec, error) {
	spec := ir.ProviderSpec{}
	providerName, err := requiredStringField(m, "provider")
	if err != nil {
		return spec, fmt.Errorf("provider name required")
	}
	spec.Provider = providerName
	for name, destination := range map[string]*string{
		"id": &spec.ID, "run": &spec.Run, "report": &spec.Report, "prompt": &spec.Prompt,
		"working_directory": &spec.WorkingDirectory, "timeout": &spec.Timeout,
		"evidence_class": &spec.EvidenceClass,
	} {
		if err := optionalStringField(m, name, destination); err != nil {
			return spec, err
		}
	}
	for name, destination := range map[string]*map[string]any{
		"result": &spec.Result, "expect": &spec.Expect, "assert": &spec.Assert,
		"match": &spec.Match, "allow": &spec.Allow, "configuration": &spec.Configuration,
	} {
		if err := optionalMapField(m, name, destination); err != nil {
			return spec, err
		}
	}
	for name, destination := range map[string]*[]string{
		"inherit_environment": &spec.InheritEnv, "allowed": &spec.Allowed,
		"forbidden": &spec.Forbidden, "paths": &spec.Paths, "inputs": &spec.Inputs,
		"outputs": &spec.Outputs, "artifacts": &spec.Artifacts, "depends_on": &spec.DependsOn,
	} {
		if err := optionalStringsField(m, name, destination); err != nil {
			return spec, err
		}
	}
	if v, ok := m["environment"]; ok {
		env, err := stringMap(v)
		if err != nil {
			return spec, err
		}
		spec.Environment = env
	}
	if v, ok := m["retry"]; ok {
		retry, err := retryValue(v)
		if err != nil {
			return spec, err
		}
		spec.Retry = retry
	}
	if raw, ok := m["exclusive"]; ok {
		value, valid := raw.(bool)
		if !valid {
			return spec, fmt.Errorf("provider field %q must be a boolean", "exclusive")
		}
		spec.Exclusive = value
	}
	known := map[string]bool{
		"provider": true, "id": true, "run": true, "report": true,
		"result": true, "expect": true, "assert": true, "match": true, "allow": true,
		"allowed": true, "forbidden": true, "paths": true,
		"prompt": true, "working_directory": true, "inherit_environment": true,
		"environment": true, "timeout": true, "retry": true, "inputs": true,
		"outputs": true, "artifacts": true, "depends_on": true, "exclusive": true,
		"evidence_class": true, "configuration": true,
	}
	for k := range m {
		if !known[k] {
			return spec, fmt.Errorf("unknown provider field %q", k)
		}
	}
	return spec, nil
}

func requiredStringField(values map[string]any, name string) (string, error) {
	raw, ok := values[name]
	if !ok {
		return "", fmt.Errorf("provider field %q is required", name)
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", fmt.Errorf("provider field %q must be a non-empty string", name)
	}
	return value, nil
}

func optionalStringField(values map[string]any, name string, destination *string) error {
	raw, ok := values[name]
	if !ok {
		return nil
	}
	if !isScalar(raw) {
		return fmt.Errorf("provider field %q must be a string", name)
	}
	*destination = scalarString(raw)
	return nil
}

func optionalMapField(values map[string]any, name string, destination *map[string]any) error {
	raw, ok := values[name]
	if !ok {
		return nil
	}
	value, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("provider field %q must be a mapping", name)
	}
	*destination = value
	return nil
}

func optionalStringsField(values map[string]any, name string, destination *[]string) error {
	raw, ok := values[name]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("provider field %q must be a string list", name)
	}
	output := make([]string, 0, len(items))
	for _, rawItem := range items {
		if !isScalar(rawItem) {
			return fmt.Errorf("provider field %q must contain only strings", name)
		}
		output = append(output, scalarString(rawItem))
	}
	*destination = output
	return nil
}

func decodeKnown(raw string, out any) error {
	dec := yaml.NewDecoder(strings.NewReader(raw))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func stringMap(v any) (map[string]string, error) {
	raw, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("environment must be a mapping")
	}
	out := make(map[string]string, len(raw))
	for key, value := range raw {
		if !isScalar(value) {
			return nil, fmt.Errorf("environment value %q must be a string", key)
		}
		out[key] = scalarString(value)
	}
	return out, nil
}

func isScalar(value any) bool {
	switch value.(type) {
	case string, bool, int, int64, float64:
		return true
	default:
		return false
	}
}

func retryValue(v any) (ir.Retry, error) {
	raw, _ := yaml.Marshal(v)
	var retry ir.Retry
	if err := decodeKnown(string(raw), &retry); err != nil {
		return ir.Retry{}, fmt.Errorf("retry: %w", err)
	}
	return retry, nil
}

// scalarString coerces YAML scalars (including unquoted true/false/numbers) to strings.
func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", t)
	}
}

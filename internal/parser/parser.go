package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hypertrial/intentci/internal/ir"
)

var idRE = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{1,31}-[0-9]{1,8}$`)

// Diagnostic is a parse/compile diagnostic.
type Diagnostic struct {
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

func (d Diagnostic) Error() string {
	if d.Path == "" {
		return d.Message
	}
	return d.Path + ": " + d.Message
}

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
		return ir.Requirement{}, []Diagnostic{{Path: path, Message: err.Error()}}
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
		Tags []string `yaml:"tags"`
	}
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return ir.Requirement{}, []Diagnostic{{Path: path, Message: "front matter: " + err.Error()}}
	}

	req := ir.Requirement{
		ID:        meta.ID,
		Title:     meta.Title,
		Status:    meta.Status,
		Priority:  meta.Priority,
		Owners:    meta.Owners,
		DependsOn: meta.DependsOn,
		AppliesTo: ir.AppliesTo{Paths: meta.AppliesTo.Paths, Symbols: meta.AppliesTo.Symbols},
		Tags:      meta.Tags,
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
	}
	if req.Priority == "" {
		diags = append(diags, Diagnostic{Path: path, Message: "missing priority"})
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
	return req, diags
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
	for kind, body := range parts {
		var items []struct {
			ID        string `yaml:"id"`
			Statement string `yaml:"statement"`
		}
		// body may be a yaml list directly
		trimmed := strings.TrimSpace(body)
		if trimmed == "" {
			continue
		}
		if err := yaml.Unmarshal([]byte(trimmed), &items); err != nil {
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
	if err := yaml.Unmarshal([]byte(raw), &b); err != nil {
		return b, []Diagnostic{{Path: path, Message: "boundaries: " + err.Error()}}
	}
	return b, nil
}

type obligationYAML struct {
	ID        string         `yaml:"id"`
	Statement string         `yaml:"statement"`
	Required  *bool          `yaml:"required"`
	Verify    map[string]any `yaml:"verify"`
}

func parseObligations(path, raw string) ([]ir.Obligation, []Diagnostic) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```yaml")
	raw = strings.TrimPrefix(raw, "```yml")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var items []obligationYAML
	if err := yaml.Unmarshal([]byte(raw), &items); err != nil {
		return nil, []Diagnostic{{Path: path, Message: "obligations: " + err.Error()}}
	}
	var out []ir.Obligation
	var diags []Diagnostic
	for _, it := range items {
		if it.ID == "" {
			diags = append(diags, Diagnostic{Path: path, Message: "obligation missing id"})
			continue
		}
		req := true
		if it.Required != nil {
			req = *it.Required
		}
		node, err := mapToVerify(it.Verify)
		if err != nil {
			diags = append(diags, Diagnostic{Path: path, Message: it.ID + ": " + err.Error()})
			continue
		}
		out = append(out, ir.Obligation{
			ID:        it.ID,
			Statement: it.Statement,
			Required:  req,
			Verify:    node,
		})
	}
	return out, diags
}

func mapToVerify(m map[string]any) (ir.VerifyNode, error) {
	if m == nil {
		return ir.VerifyNode{}, fmt.Errorf("missing verify")
	}
	if v, ok := m["all"]; ok {
		nodes, err := toNodeList(v)
		if err != nil {
			return ir.VerifyNode{}, err
		}
		return ir.VerifyNode{All: nodes}, nil
	}
	if v, ok := m["any"]; ok {
		nodes, err := toNodeList(v)
		if err != nil {
			return ir.VerifyNode{}, err
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
	if _, ok := m["provider"]; ok {
		spec, err := toProvider(m)
		if err != nil {
			return ir.VerifyNode{}, err
		}
		return ir.VerifyNode{Provider: &spec}, nil
	}
	return ir.VerifyNode{}, fmt.Errorf("verify must contain all, any, not, or provider")
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
	if v, ok := m["provider"].(string); ok {
		spec.Provider = v
	} else {
		return spec, fmt.Errorf("provider name required")
	}
	if v, ok := m["id"].(string); ok {
		spec.ID = v
	}
	if v, ok := m["run"]; ok {
		spec.Run = scalarString(v)
	}
	if v, ok := m["report"]; ok {
		spec.Report = scalarString(v)
	}
	if v, ok := m["result"].(map[string]any); ok {
		spec.Result = v
	}
	if v, ok := m["expect"].(map[string]any); ok {
		spec.Expect = v
	}
	if v, ok := m["assert"].(map[string]any); ok {
		spec.Assert = v
	}
	spec.Allowed = stringSlice(m["allowed"])
	spec.Forbidden = stringSlice(m["forbidden"])
	spec.Paths = stringSlice(m["paths"])
	// stash unknown extras
	known := map[string]bool{
		"provider": true, "id": true, "run": true, "report": true,
		"result": true, "expect": true, "assert": true,
		"allowed": true, "forbidden": true, "paths": true,
	}
	extra := map[string]any{}
	for k, v := range m {
		if !known[k] {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		spec.Extra = extra
	}
	return spec, nil
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s := scalarString(x); s != "" || x == "" {
			out = append(out, s)
		}
	}
	return out
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

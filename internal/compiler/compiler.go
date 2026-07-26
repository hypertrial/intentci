package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/parser"
)

// Result is a compile outcome.
type Result struct {
	Document    *ir.Document
	Diagnostics []parser.Diagnostic
}

// Options configures compilation.
type Options struct {
	Root          string
	Config        *config.Config
	RequirementID string
	Strict        bool
}

var supportedProviders = map[string]bool{
	"command":  true,
	"junit":    true,
	"sarif":    true,
	"boundary": true,
	"git-diff": true,
	"json":     true,
	"manual":   true,
}

var absPath = filepath.Abs
var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile
var computeHashes = func(doc *ir.Document) error { return doc.ComputeHashes() }

// Compile parses requirements and produces canonical IR.
func Compile(opt Options) (*Result, error) {
	if opt.Config == nil {
		cfg, err := config.Load(opt.Root)
		if err != nil {
			return &Result{Diagnostics: []parser.Diagnostic{{Message: err.Error()}}}, err
		}
		opt.Config = cfg
	}
	files, err := discover(opt.Root, opt.Config.Requirements.Paths)
	if err != nil {
		return &Result{Diagnostics: []parser.Diagnostic{{Message: err.Error()}}}, err
	}
	sort.Strings(files)

	var diags []parser.Diagnostic
	var reqs []ir.Requirement
	seen := map[string]string{}
	for _, f := range files {
		rel, _ := filepath.Rel(opt.Root, f)
		req, d := parser.ParseFile(f)
		if rel != "" {
			req.SourcePath = filepath.ToSlash(rel)
		}
		diags = append(diags, d...)
		if opt.RequirementID != "" && req.ID != opt.RequirementID {
			continue
		}
		if req.ID != "" {
			if prev, ok := seen[req.ID]; ok {
				diags = append(diags, parser.Diagnostic{
					Path:    req.SourcePath,
					Message: fmt.Sprintf("duplicate requirement id %q (also in %s)", req.ID, prev),
				})
			} else {
				seen[req.ID] = req.SourcePath
			}
			reqs = append(reqs, req)
		}
	}

	doc := &ir.Document{
		SchemaVersion: ir.SchemaVersion,
		Project:       opt.Config.Project.Name,
		Requirements:  reqs,
	}
	diags = append(diags, validateGraph(doc)...)
	diags = append(diags, validateProviders(doc)...)
	diags = append(diags, validateBoundaries(doc)...)

	if err := computeHashes(doc); err != nil {
		return &Result{Document: doc, Diagnostics: diags}, err
	}

	res := &Result{Document: doc, Diagnostics: diags}
	if opt.Strict && len(diags) > 0 {
		return res, fmt.Errorf("compile failed with %d diagnostic(s)", len(diags))
	}
	if hasErrors(diags) {
		return res, fmt.Errorf("compile failed with %d diagnostic(s)", len(diags))
	}
	return res, nil
}

func hasErrors(d []parser.Diagnostic) bool {
	return len(d) > 0
}

func discover(root string, patterns []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, pat := range patterns {
		p := pat
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		matches, err := doublestar.FilepathGlob(p)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(m), ".md") {
				continue
			}
			abs, err := absPath(m)
			if err != nil {
				continue
			}
			if !seen[abs] {
				seen[abs] = true
				out = append(out, abs)
			}
		}
	}
	return out, nil
}

func validateGraph(doc *ir.Document) []parser.Diagnostic {
	var diags []parser.Diagnostic
	ids := map[string]bool{}
	for _, r := range doc.Requirements {
		ids[r.ID] = true
	}
	for _, r := range doc.Requirements {
		for _, dep := range r.DependsOn {
			if !ids[dep] {
				diags = append(diags, parser.Diagnostic{
					Path:    r.SourcePath,
					Message: fmt.Sprintf("depends_on unknown requirement %q", dep),
				})
			}
		}
		oblIDs := map[string]bool{}
		for _, o := range r.Obligations {
			if oblIDs[o.ID] {
				diags = append(diags, parser.Diagnostic{
					Path:    r.SourcePath,
					Message: fmt.Sprintf("duplicate obligation id %q", o.ID),
				})
			}
			oblIDs[o.ID] = true
		}
	}
	// cycles
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(id string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		r := doc.RequirementByID(id)
		if r != nil {
			for _, dep := range r.DependsOn {
				if visit(dep) {
					return true
				}
			}
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for _, r := range doc.Requirements {
		if visit(r.ID) {
			diags = append(diags, parser.Diagnostic{
				Path:    r.SourcePath,
				Message: "cyclic requirement dependency involving " + r.ID,
			})
			break
		}
	}
	return diags
}

func validateProviders(doc *ir.Document) []parser.Diagnostic {
	var diags []parser.Diagnostic
	var walk func(path, obl string, n ir.VerifyNode)
	walk = func(path, obl string, n ir.VerifyNode) {
		if n.Provider != nil {
			p := n.Provider.Provider
			if !supportedProviders[p] {
				diags = append(diags, parser.Diagnostic{
					Path:    path,
					Message: fmt.Sprintf("%s: unsupported provider %q", obl, p),
				})
			}
			if p == "command" && strings.TrimSpace(n.Provider.Run) == "" {
				diags = append(diags, parser.Diagnostic{
					Path:    path,
					Message: obl + ": command provider requires run",
				})
			}
			if p == "boundary" && len(n.Provider.Forbidden) == 0 && len(n.Provider.Allowed) == 0 {
				diags = append(diags, parser.Diagnostic{
					Path:    path,
					Message: obl + ": boundary provider requires allowed or forbidden",
				})
			}
		}
		for _, c := range n.All {
			walk(path, obl, c)
		}
		for _, c := range n.Any {
			walk(path, obl, c)
		}
		if n.Not != nil {
			walk(path, obl, *n.Not)
		}
	}
	for _, r := range doc.Requirements {
		for _, o := range r.Obligations {
			walk(r.SourcePath, o.ID, o.Verify)
		}
	}
	return diags
}

func validateBoundaries(doc *ir.Document) []parser.Diagnostic {
	var diags []parser.Diagnostic
	for _, r := range doc.Requirements {
		for _, a := range r.Boundaries.Allowed {
			for _, f := range r.Boundaries.Forbidden {
				if a == f {
					diags = append(diags, parser.Diagnostic{
						Path:    r.SourcePath,
						Message: fmt.Sprintf("contradictory boundary path %q in allowed and forbidden", a),
					})
				}
			}
		}
	}
	return diags
}

// WriteIR writes the document JSON to path.
func WriteIR(doc *ir.Document, path string) error {
	if err := mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := ir.CanonicalJSON(doc)
	if err != nil {
		return err
	}
	return writeFile(path, append(b, '\n'), 0o644)
}

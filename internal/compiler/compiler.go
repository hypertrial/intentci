package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/oklog/ulid/v2"

	"github.com/hypertrial/intentci/internal/config"
	"github.com/hypertrial/intentci/internal/ir"
	"github.com/hypertrial/intentci/internal/parser"
	"github.com/hypertrial/intentci/internal/provider"
	"github.com/hypertrial/intentci/internal/security"
	appschema "github.com/hypertrial/intentci/pkg/schema"
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

var lookPath = exec.LookPath

var absPath = filepath.Abs
var mkdirAll = os.MkdirAll
var writeFile = os.WriteFile
var renameFile = os.Rename
var removeFile = os.Remove
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
	workingDirectory, workingErr := security.ResolveInside(opt.Root, opt.Config.Verification.WorkingDirectory)
	workingInfo, statErr := os.Stat(workingDirectory)
	if workingErr != nil || statErr != nil || !workingInfo.IsDir() {
		diags = append(diags, parser.Diagnostic{
			Message: "verification.working_directory does not exist or is unsafe",
		})
	}
	reqs := make([]ir.Requirement, 0)
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
	diags = append(diags, validateRequirements(doc)...)
	diags = append(diags, validateProviders(opt.Root, doc)...)
	diags = append(diags, validateBoundaries(doc)...)
	diags = append(diags, compilerWarnings(doc, opt.Config)...)

	if err := computeHashes(doc); err != nil {
		return &Result{Document: doc, Diagnostics: diags}, err
	}
	if err := appschema.Validate("ir", doc); err != nil {
		diags = append(diags, parser.Diagnostic{Message: err.Error()})
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
	for _, diag := range d {
		if diag.Severity != parser.SeverityWarning {
			return true
		}
	}
	return false
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
		for _, o := range r.Obligations {
			for _, dep := range o.DependsOn {
				if !oblIDs[dep] {
					diags = append(diags, parser.Diagnostic{
						Path: r.SourcePath, Message: fmt.Sprintf("%s depends_on unknown obligation %q", o.ID, dep),
					})
				}
			}
		}
		if cycle := obligationCycle(r.Obligations); cycle != "" {
			diags = append(diags, parser.Diagnostic{
				Path: r.SourcePath, Message: "cyclic obligation dependency involving " + cycle,
			})
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

func obligationCycle(obligations []ir.Obligation) string {
	byID := make(map[string]ir.Obligation, len(obligations))
	for _, obligation := range obligations {
		byID[obligation.ID] = obligation
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		for _, dep := range byID[id].DependsOn {
			if _, ok := byID[dep]; ok && visit(dep) {
				return true
			}
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for id := range byID {
		if visit(id) {
			return id
		}
	}
	return ""
}

func validateRequirements(doc *ir.Document) []parser.Diagnostic {
	var diags []parser.Diagnostic
	for _, requirement := range doc.Requirements {
		for _, pattern := range append(append([]string{}, requirement.AppliesTo.Paths...), append(requirement.Boundaries.Allowed, requirement.Boundaries.Forbidden...)...) {
			if !validPattern(pattern) {
				diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: fmt.Sprintf("invalid path pattern %q", pattern)})
			}
		}
		if requirement.Timeout != "" {
			if _, err := config.ParseDuration(requirement.Timeout); err != nil {
				diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: "timeout: " + err.Error()})
			}
		}
		for _, obligation := range requirement.Obligations {
			if !oneOf(obligation.EvidenceClass, "", "deterministic", "probabilistic", "human", "informational") {
				diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: obligation.ID + ": invalid evidence_class"})
			}
			if !oneOf(obligation.Severity, "", "error", "warning", "note") {
				diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: obligation.ID + ": invalid severity"})
			}
			if obligation.ConfidenceThreshold != nil && (*obligation.ConfidenceThreshold < 0 || *obligation.ConfidenceThreshold > 1) {
				diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: obligation.ID + ": confidence_threshold must be between 0 and 1"})
			}
			if obligation.ConfidenceThreshold != nil && obligation.EvidenceClass != "probabilistic" {
				diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: obligation.ID + ": confidence_threshold requires probabilistic evidence"})
			}
			if obligation.Timeout != "" {
				if _, err := config.ParseDuration(obligation.Timeout); err != nil {
					diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: obligation.ID + ": timeout: " + err.Error()})
				}
			}
			diags = append(diags, validateRetry(requirement.SourcePath, obligation.ID, obligation.Retry)...)
			for _, platform := range obligation.Platforms {
				if !oneOf(platform, "linux", "darwin") {
					diags = append(diags, parser.Diagnostic{Path: requirement.SourcePath, Message: fmt.Sprintf("%s: unsupported platform %q", obligation.ID, platform)})
				}
			}
		}
	}
	return diags
}

func validateProviders(root string, doc *ir.Document) []parser.Diagnostic {
	var diags []parser.Diagnostic
	registry := provider.DefaultRegistry()
	var walk func(path, obl string, n ir.VerifyNode)
	walk = func(path, obl string, n ir.VerifyNode) {
		if n.Provider != nil {
			p := n.Provider.Provider
			if !validLocalID(p) {
				diags = append(diags, parser.Diagnostic{
					Path: path, Message: fmt.Sprintf("%s: invalid provider name %q", obl, p),
				})
			}
			if n.Provider.ID != "" && !validLocalID(n.Provider.ID) {
				diags = append(diags, parser.Diagnostic{
					Path: path, Message: fmt.Sprintf("%s: invalid verifier id %q", obl, n.Provider.ID),
				})
			}
			if !supportedProviders[p] {
				if _, err := lookPath("intentci-provider-" + p); err != nil {
					diags = append(diags, parser.Diagnostic{
						Path: path, Message: fmt.Sprintf("%s: unsupported provider %q", obl, p),
					})
				}
			} else if implementation, ok := registry.Get(p); ok {
				for _, diagnostic := range implementation.Validate(*n.Provider) {
					diags = append(diags, parser.Diagnostic{Path: path, Message: obl + ": " + diagnostic.Message})
				}
			}
			for _, pattern := range providerPaths(*n.Provider) {
				if !validPattern(pattern) {
					diags = append(diags, parser.Diagnostic{Path: path, Message: fmt.Sprintf("%s: invalid provider path %q", obl, pattern)})
				}
			}
			if n.Provider.WorkingDirectory != "" && !validRelativePath(n.Provider.WorkingDirectory) {
				diags = append(diags, parser.Diagnostic{Path: path, Message: obl + ": invalid working_directory"})
			} else if n.Provider.WorkingDirectory != "" {
				resolved, err := security.ResolveInside(root, n.Provider.WorkingDirectory)
				info, statErr := os.Stat(resolved)
				if err != nil || statErr != nil || !info.IsDir() {
					diags = append(diags, parser.Diagnostic{Path: path, Message: obl + ": working_directory does not exist or is unsafe"})
				}
			}
			if n.Provider.Report != "" && !validRelativePath(n.Provider.Report) {
				diags = append(diags, parser.Diagnostic{Path: path, Message: obl + ": invalid report path"})
			} else if n.Provider.Report != "" && n.Provider.Run == "" {
				resolved, err := security.ResolveInside(root, n.Provider.Report)
				info, statErr := os.Stat(resolved)
				if err != nil || statErr != nil || !info.Mode().IsRegular() {
					diags = append(diags, parser.Diagnostic{Path: path, Message: obl + ": referenced report does not exist or is unsafe"})
				}
			}
			for _, pattern := range n.Provider.InheritEnv {
				if _, err := filepath.Match(pattern, "INTENTCI"); err != nil {
					diags = append(diags, parser.Diagnostic{
						Path: path, Message: fmt.Sprintf("%s: invalid inherited environment pattern %q", obl, pattern),
					})
				}
			}
			for name := range n.Provider.Environment {
				if name == "" || strings.ContainsAny(name, "=\x00") {
					diags = append(diags, parser.Diagnostic{
						Path: path, Message: fmt.Sprintf("%s: invalid environment name %q", obl, name),
					})
				}
			}
			if n.Provider.Timeout != "" {
				if _, err := config.ParseDuration(n.Provider.Timeout); err != nil {
					diags = append(diags, parser.Diagnostic{Path: path, Message: obl + ": timeout: " + err.Error()})
				}
			}
			diags = append(diags, validateRetry(path, obl, n.Provider.Retry)...)
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
			seen := map[string]bool{}
			collectProviderIDs(o.Verify, func(id string) {
				if id != "" && seen[id] {
					diags = append(diags, parser.Diagnostic{Path: r.SourcePath, Message: fmt.Sprintf("%s: duplicate verifier id %q", o.ID, id)})
				}
				seen[id] = true
			})
		}
		diags = append(diags, validateVerifierGraph(r)...)
	}
	return diags
}

func validLocalID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func validateVerifierGraph(requirement ir.Requirement) []parser.Diagnostic {
	var diagnostics []parser.Diagnostic
	specs := map[string]ir.ProviderSpec{}
	dependencies := map[string][]string{}
	for _, obligation := range requirement.Obligations {
		var walk func(ir.VerifyNode)
		walk = func(node ir.VerifyNode) {
			if node.Provider != nil && node.Provider.ID != "" {
				if prior, ok := specs[node.Provider.ID]; ok {
					left, _ := ir.CanonicalJSON(prior)
					right, _ := ir.CanonicalJSON(*node.Provider)
					if string(left) != string(right) {
						diagnostics = append(diagnostics, parser.Diagnostic{
							Path:    requirement.SourcePath,
							Message: fmt.Sprintf("verifier id %q is reused with incompatible configuration", node.Provider.ID),
						})
					}
				} else {
					specs[node.Provider.ID] = *node.Provider
					dependencies[node.Provider.ID] = append([]string{}, node.Provider.DependsOn...)
				}
			}
			for _, child := range node.All {
				walk(child)
			}
			for _, child := range node.Any {
				walk(child)
			}
			if node.Not != nil {
				walk(*node.Not)
			}
		}
		walk(obligation.Verify)
	}
	for id, values := range dependencies {
		for _, dependency := range values {
			if _, ok := specs[dependency]; !ok {
				diagnostics = append(diagnostics, parser.Diagnostic{
					Path:    requirement.SourcePath,
					Message: fmt.Sprintf("verifier %q depends_on unknown verifier %q", id, dependency),
				})
			}
		}
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		for _, dependency := range dependencies[id] {
			if visit(dependency) {
				return true
			}
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for id := range dependencies {
		if visit(id) {
			diagnostics = append(diagnostics, parser.Diagnostic{
				Path: requirement.SourcePath, Message: "cyclic verifier dependency involving " + id,
			})
			break
		}
	}
	return diagnostics
}

func collectProviderIDs(node ir.VerifyNode, fn func(string)) {
	if node.Provider != nil {
		fn(node.Provider.ID)
	}
	for _, child := range node.All {
		collectProviderIDs(child, fn)
	}
	for _, child := range node.Any {
		collectProviderIDs(child, fn)
	}
	if node.Not != nil {
		collectProviderIDs(*node.Not, fn)
	}
}

func providerPaths(spec ir.ProviderSpec) []string {
	var paths []string
	paths = append(paths, spec.Allowed...)
	paths = append(paths, spec.Forbidden...)
	paths = append(paths, spec.Paths...)
	paths = append(paths, spec.Inputs...)
	paths = append(paths, spec.Outputs...)
	paths = append(paths, spec.Artifacts...)
	return paths
}

func validPattern(pattern string) bool {
	return validRelativePath(pattern) && doublestar.ValidatePattern(filepath.ToSlash(pattern))
}

func validRelativePath(path string) bool {
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func validateRetry(path, owner string, retry ir.Retry) []parser.Diagnostic {
	if retry.Attempts < 0 {
		return []parser.Diagnostic{{Path: path, Message: owner + ": retry.attempts must be >= 0"}}
	}
	if retry.Backoff != "" {
		if _, err := config.ParseDuration(retry.Backoff); err != nil {
			return []parser.Diagnostic{{Path: path, Message: owner + ": retry.backoff: " + err.Error()}}
		}
	}
	return nil
}

func oneOf(got string, values ...string) bool {
	for _, value := range values {
		if got == value {
			return true
		}
	}
	return false
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

func compilerWarnings(doc *ir.Document, cfg *config.Config) []parser.Diagnostic {
	var diagnostics []parser.Diagnostic
	status := map[string]string{}
	for _, requirement := range doc.Requirements {
		status[requirement.ID] = requirement.Status
	}
	for _, requirement := range doc.Requirements {
		warn := func(message string) {
			diagnostics = append(diagnostics, parser.Diagnostic{
				Severity: parser.SeverityWarning, Path: requirement.SourcePath, Message: message,
			})
		}
		if len(requirement.AppliesTo.Paths) == 0 {
			warn("requirement has no applies_to.paths mapping")
		}
		if len(requirement.Owners) == 0 {
			warn("requirement has no owner")
		}
		for _, pattern := range append(append([]string{}, requirement.Boundaries.Allowed...), requirement.Boundaries.Forbidden...) {
			if pattern == "**" || pattern == "**/*" {
				warn("requirement uses a broad file boundary " + fmt.Sprintf("%q", pattern))
			}
			if security.IsTestPath(pattern) && containsPattern(requirement.Boundaries.Allowed, pattern) {
				warn("test path is inside the allowed implementation boundary: " + pattern)
			}
		}
		for _, dependency := range requirement.DependsOn {
			if status[dependency] == "disabled" {
				warn("active requirement references disabled requirement " + dependency)
			}
		}
		for _, obligation := range requirement.Obligations {
			var specs []ir.ProviderSpec
			collectSpecs(obligation.Verify, &specs)
			if len(specs) == 0 {
				continue
			}
			probabilisticOnly := true
			exitCodeOnly := true
			for _, spec := range specs {
				class := firstNonEmpty(spec.EvidenceClass, obligation.EvidenceClass, "deterministic")
				if class != "probabilistic" {
					probabilisticOnly = false
				}
				if spec.Provider != "command" || hasOutputExpectation(spec.Result) {
					exitCodeOnly = false
				}
				if spec.Provider == "command" && spec.Timeout == "" && obligation.Timeout == "" && requirement.Timeout == "" && cfg.Verification.DefaultTimeout == "" {
					warn(obligation.ID + ": command has no timeout")
				}
				run := strings.ToLower(spec.Run)
				configuration, _ := ir.CanonicalJSON(spec.Configuration)
				if literalCredential(spec.Run) || literalCredential(string(configuration)) ||
					environmentCredential(spec.Environment) {
					warn(obligation.ID + ": verifier appears to contain a literal credential")
				}
				if strings.Contains(run, "git add") || strings.Contains(run, "git commit") ||
					strings.Contains(run, "sed -i") || strings.Contains(run, " >") || strings.Contains(run, "tee ") {
					warn(obligation.ID + ": verification command may modify tracked files")
				}
			}
			if probabilisticOnly {
				warn(obligation.ID + ": obligation has only probabilistic evidence")
			}
			if exitCodeOnly {
				warn(obligation.ID + ": obligation relies only on command exit codes")
			}
		}
	}
	return diagnostics
}

func environmentCredential(environment map[string]string) bool {
	for name, value := range environment {
		lower := strings.ToLower(name)
		if value != "" && (strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "password") || strings.Contains(lower, "api_key") ||
			strings.Contains(lower, "api-key") || strings.HasSuffix(lower, "_key")) {
			return true
		}
	}
	return false
}

func literalCredential(command string) bool {
	lower := strings.ToLower(command)
	for _, marker := range []string{
		"token=", "token:", "secret=", "secret:", "password=", "password:",
		"api_key=", "api_key:", "api-key=", "api-key:",
		`"token":"`, `"secret":"`, `"password":"`, `"api_key":"`,
	} {
		index := strings.Index(lower, marker)
		if index < 0 {
			continue
		}
		value := strings.TrimLeft(strings.TrimSpace(command[index+len(marker):]), `"'`)
		if value != "" && !strings.HasPrefix(value, "$") && !strings.HasPrefix(value, "${") &&
			!strings.HasPrefix(value, "[REDACTED]") {
			return true
		}
	}
	return false
}

func collectSpecs(node ir.VerifyNode, specs *[]ir.ProviderSpec) {
	if node.Provider != nil {
		*specs = append(*specs, *node.Provider)
	}
	for _, child := range node.All {
		collectSpecs(child, specs)
	}
	for _, child := range node.Any {
		collectSpecs(child, specs)
	}
	if node.Not != nil {
		collectSpecs(*node.Not, specs)
	}
}

func hasOutputExpectation(result map[string]any) bool {
	if result == nil {
		return false
	}
	return result["stdout"] != nil || result["stderr"] != nil
}

func containsPattern(patterns []string, want string) bool {
	for _, pattern := range patterns {
		if pattern == want {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// WriteIR writes the document JSON to path.
func WriteIR(doc *ir.Document, path string) error {
	if doc.Requirements == nil {
		doc.Requirements = make([]ir.Requirement, 0)
	}
	if doc.Hash == "" {
		if err := doc.ComputeHashes(); err != nil {
			return err
		}
	}
	if err := appschema.Validate("ir", doc); err != nil {
		return err
	}
	if err := mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := ir.CanonicalJSON(doc)
	temporary := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp-"+ulid.Make().String())
	if err := writeFile(temporary, append(b, '\n'), 0o644); err != nil {
		return err
	}
	defer removeFile(temporary)
	return renameFile(temporary, path)
}

package pkgpack

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// HealthCheckHint describes how to verify a package is healthy on a node.
type HealthCheckHint struct {
	Unit string `yaml:"unit"` // systemd unit that must be active
	Port int    `yaml:"port"` // TCP port that must be listening (0 = skip)
}

// SpecMetadata contains optional package metadata from the spec's metadata: section.
type SpecMetadata struct {
	Kind        string   // "service", "infrastructure", "application", "command"
	Description string
	Keywords    []string
	License     string

	// Day 1 orchestration fields — drive profile-aware, dependency-gated convergence.
	ProvidesCapabilities     []string         // capabilities this package gives the node (e.g. "local-db", "object-store")
	InstallDependencies      []string         // packages that must be installed before this one
	RuntimeLocalDependencies []string         // packages that must be healthy on the same node before this starts
	HealthCheck              *HealthCheckHint // how to verify this package is healthy

	// Catalog fields — drive dynamic component catalog in the cluster controller.
	Profiles    []string // profiles that include this component (e.g. "core", "compute")
	Priority    int      // start order (lower = starts first, stops last); 0 = default (1000)
	InstallMode string   // "repository" | "day0_join"
	ManagedUnit bool     // included in profileUnitMap for unit actions
	SystemdUnit string   // override systemd unit name (auto-derived from spec if empty)

	// Extra binaries to include alongside the main exec (e.g. helper tools).
	ExtraBinaries []string

	// Build hints — control how the builder treats this package.
	Entrypoint  string   // Override exec name (e.g. "noop" for OS-managed packages, "globularcli" for renamed binaries).
	InstallBins *bool    // Override: false to skip bin/ extraction (OS-managed packages like scylladb).
	BundleDebs  []string // OS package names to download as .deb at build time for offline install.
}

// ScriptFile describes a script to embed in a package.
type ScriptFile struct {
	Name       string // filename (e.g. "post-install.sh")
	SourcePath string // absolute path on disk
}

// ExtraBinary describes an additional binary to include in the package.
type ExtraBinary struct {
	Name string // binary name (e.g. "globular-upgrader")
	Path string // absolute path on disk
}

// SpecInfo contains derived data from a spec.
type SpecInfo struct {
	SpecPath      string
	SpecFile      string
	ServiceName   string
	ExecName      string
	ExecPath      string
	ExtraBinaries []ExtraBinary
	ConfigDirs    []string
	Systemd       []SystemdFile
	Scripts       []ScriptFile
	DebPaths      []string // .deb files to bundle in debs/ directory
	Metadata      SpecMetadata
}

type SystemdFile struct {
	Name       string // relative path within systemd/
	SourcePath string // optional: copy from this path
	Content    []byte // optional: inline content
}

type AssetRoots struct {
	BinRoot     string
	ConfigRoot  string
	ScriptsRoot string
}

type ScanOptions struct {
	SkipMissingConfig  bool
	SkipMissingSystemd bool
}

// ScanSpec reads a spec file and derives service metadata, executable, config, and systemd assets.
// It first parses and validates the spec against the PackageSpec schema, catching structural
// errors early before attempting asset discovery.
func ScanSpec(specPath string, roots AssetRoots, opts ScanOptions) (*SpecInfo, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}

	// Parse into typed struct and validate schema.
	parsed, parseErr := ParseSpecBytes(data, specPath)
	if parseErr == nil {
		if errs := ValidateSpec(parsed, specPath); len(errs) > 0 {
			// Log warnings but don't fail — existing specs may have minor issues.
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "WARN %v\n", e)
			}
		}
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	serviceName := deriveServiceName(specPath, doc)

	// Check for top-level entrypoint field (e.g. "bin/mc", "bin/globularcli").
	topEntrypoint := lookupString(doc, "entrypoint")

	// Check for metadata.entrypoint (e.g. "noop" for OS-managed packages).
	metaEntrypoint := lookupString(doc, "metadata", "entrypoint")

	var execName string
	switch {
	case metaEntrypoint == "noop":
		// OS-managed package — use noop binary from bin root.
		execName = "noop"
	case metaEntrypoint != "":
		// Metadata entrypoint override (bare name or bin/name).
		execName = strings.TrimPrefix(metaEntrypoint, "bin/")
	case topEntrypoint != "":
		// Top-level entrypoint field (e.g. "bin/yt-dlp").
		execName = strings.TrimPrefix(topEntrypoint, "bin/")
	default:
		// Standard exec discovery from spec content.
		var discoverErr error
		execName, discoverErr = deriveExecName(doc, roots, serviceName)
		if discoverErr != nil {
			return nil, fmt.Errorf("spec %s: %w", specPath, discoverErr)
		}
	}
	execPath := filepath.Join(roots.BinRoot, execName)

	configDirs, err := discoverConfigDirs(doc, roots, serviceName, opts.SkipMissingConfig)
	if err != nil {
		return nil, fmt.Errorf("spec %s: %w", specPath, err)
	}

	systemdFiles, err := discoverSystemdUnits(doc, roots, serviceName, configDirs, opts.SkipMissingSystemd)
	if err != nil {
		return nil, fmt.Errorf("spec %s: %w", specPath, err)
	}

	meta := extractMetadata(doc, specPath)

	scripts := discoverScripts(roots, serviceName)

	// Discover extra binaries from metadata.
	var extraBins []ExtraBinary
	for _, name := range meta.ExtraBinaries {
		binPath := filepath.Join(roots.BinRoot, name)
		if _, err := os.Stat(binPath); err == nil {
			extraBins = append(extraBins, ExtraBinary{Name: name, Path: binPath})
		} else {
			return nil, fmt.Errorf("spec %s: extra binary %q not found in %s", specPath, name, roots.BinRoot)
		}
	}

	return &SpecInfo{
		SpecPath:      specPath,
		SpecFile:      filepath.Base(specPath),
		ServiceName:   serviceName,
		ExecName:      execName,
		ExecPath:      execPath,
		ExtraBinaries: extraBins,
		ConfigDirs:    configDirs,
		Systemd:       systemdFiles,
		Scripts:        scripts,
		Metadata:      meta,
	}, nil
}

// extractMetadata reads the optional metadata: section from the spec YAML.
func extractMetadata(doc map[string]any, specPath string) SpecMetadata {
	var meta SpecMetadata

	// Derive default kind from filename
	base := filepath.Base(specPath)
	if strings.HasSuffix(base, "_cmd.yaml") || strings.HasSuffix(base, "_command.yaml") {
		meta.Kind = "command"
	} else {
		meta.Kind = "service"
	}

	m := lookupMap(doc, "metadata")
	if m == nil {
		return meta
	}

	if kind := lookupString(m, "kind"); kind != "" {
		meta.Kind = strings.ToLower(kind)
	}
	if desc := lookupString(m, "description"); desc != "" {
		meta.Description = desc
	}
	if lic := lookupString(m, "license"); lic != "" {
		meta.License = lic
	}

	// keywords can be a list of strings
	if kw, ok := m["keywords"]; ok {
		switch v := kw.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					meta.Keywords = append(meta.Keywords, s)
				}
			}
		case string:
			for _, s := range strings.Split(v, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					meta.Keywords = append(meta.Keywords, s)
				}
			}
		}
	}

	// Day 1 orchestration fields
	meta.ProvidesCapabilities = lookupStringList(m, "provides_capabilities")
	meta.InstallDependencies = lookupStringList(m, "install_dependencies")
	meta.RuntimeLocalDependencies = lookupStringList(m, "runtime_local_dependencies")

	if hc := lookupMap(m, "health_check"); hc != nil {
		hint := &HealthCheckHint{}
		if u := lookupString(hc, "unit"); u != "" {
			hint.Unit = u
		}
		if p, ok := hc["port"]; ok {
			switch v := p.(type) {
			case int:
				hint.Port = v
			case float64:
				hint.Port = int(v)
			}
		}
		if hint.Unit != "" || hint.Port != 0 {
			meta.HealthCheck = hint
		}
	}

	// Catalog fields
	meta.Profiles = lookupStringList(m, "profiles")
	meta.Priority = lookupInt(m, "priority")
	if im := lookupString(m, "install_mode"); im != "" {
		meta.InstallMode = im
	}
	meta.ManagedUnit = lookupBool(m, "managed_unit")
	if su := lookupString(m, "systemd_unit"); su != "" {
		meta.SystemdUnit = su
	}
	meta.ExtraBinaries = lookupStringList(m, "extra_binaries")

	// Build hint fields
	if ep := lookupString(m, "entrypoint"); ep != "" {
		meta.Entrypoint = ep
	}
	if ib, ok := m["install_bins"]; ok {
		if b, ok := ib.(bool); ok {
			meta.InstallBins = &b
		}
	}
	meta.BundleDebs = lookupStringList(m, "bundle_debs")

	return meta
}

func deriveServiceName(specPath string, doc map[string]any) string {
	if name := lookupString(doc, "metadata", "name"); name != "" {
		return normalizeServiceName(name)
	}
	if name := lookupString(doc, "service", "name"); name != "" {
		return normalizeServiceName(name)
	}
	return normalizeServiceName(serviceNameFromFile(specPath))
}

func serviceNameFromFile(specPath string) string {
	base := filepath.Base(specPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.TrimSuffix(name, "_service")
	name = strings.TrimSuffix(name, "-service")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func normalizeServiceName(input string) string {
	name := strings.ToLower(strings.TrimSpace(input))
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func deriveExecName(doc map[string]any, roots AssetRoots, serviceName string) (string, error) {
	var candidates []string

	// direct fields
	for _, key := range []string{"exec", "bin", "command", "entrypoint"} {
		if value := lookupString(doc, key); value != "" {
			candidates = append(candidates, execFromCommand(value))
		}
	}
	if service := lookupMap(doc, "service"); service != nil {
		for _, key := range []string{"exec", "bin", "command", "entrypoint"} {
			if value := lookupString(service, key); value != "" {
				candidates = append(candidates, execFromCommand(value))
			}
		}
		if process := lookupMap(service, "process"); process != nil {
			if value := lookupString(process, "exec"); value != "" {
				candidates = append(candidates, execFromCommand(value))
			}
		}
	}

	// scan all strings for known bin patterns
	for _, s := range collectStrings(doc) {
		if exec := execFromBinString(s); exec != "" {
			candidates = append(candidates, exec)
		}
	}

	// fallbacks based on service name
	fallbackNames := []string{
		serviceName,
		strings.ReplaceAll(serviceName, "-", ""),
		strings.ReplaceAll(serviceName, "-", "_"),
		strings.ReplaceAll(serviceName, "_", "-"),
	}
	candidates = append(candidates, fallbackNames...)

	unique := uniqueStrings(candidates)
	if len(unique) == 0 {
		return "", errors.New("no executable candidates found")
	}

	var present []string
	for _, name := range unique {
		if name == "" {
			continue
		}
		binPath := filepath.Join(roots.BinRoot, name)
		if _, err := os.Stat(binPath); err == nil {
			present = append(present, name)
		}
	}

	if len(present) == 0 {
		return "", fmt.Errorf("no executable found in %s among candidates: %s", roots.BinRoot, strings.Join(unique, ", "))
	}
	if len(present) > 1 {
		return "", fmt.Errorf("multiple executable candidates found: %s", strings.Join(present, ", "))
	}
	return present[0], nil
}

func discoverConfigDirs(doc map[string]any, roots AssetRoots, serviceName string, skipMissing bool) ([]string, error) {
	var candidates []string
	var missing []string

	for _, s := range collectStrings(doc) {
		if key := configKeyFromString(s); key != "" {
			candidates = append(candidates, key)
		}
	}

	baseCandidates := []string{serviceName, strings.ReplaceAll(serviceName, "-", "_")}
	candidates = append(candidates, baseCandidates...)
	candidates = uniqueStrings(candidates)

	var configDirs []string
	for _, name := range candidates {
		if name == "" {
			continue
		}
		dir := filepath.Join(roots.ConfigRoot, name)
		info, err := os.Stat(dir)
		if err != nil {
			if !skipMissing && errors.Is(err, os.ErrNotExist) {
				missing = append(missing, dir)
			}
			continue
		}
		if info.IsDir() {
			configDirs = append(configDirs, dir)
		}
	}

	if len(configDirs) == 0 && !skipMissing && len(missing) > 0 {
		return nil, fmt.Errorf("config directories not found: %s", strings.Join(missing, ", "))
	}
	return configDirs, nil
}

func discoverSystemdUnits(doc map[string]any, roots AssetRoots, serviceName string, configDirs []string, skipMissing bool) ([]SystemdFile, error) {
	referenced := collectSystemdReferences(doc)
	found := make(map[string]SystemdFile)

	// Inline units
	for name, content := range collectInlineUnits(doc) {
		base := path.Base(name)
		found[base] = SystemdFile{Name: base, Content: []byte(content)}
	}

	// From config/systemd folders
	candidates := []string{
		filepath.Join(roots.ConfigRoot, serviceName, "systemd"),
		filepath.Join(roots.ConfigRoot, strings.ReplaceAll(serviceName, "-", "_"), "systemd"),
	}
	candidates = append(candidates, configDirs...)
	candidates = uniquePathsWithSystemd(candidates)

	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(d.Name(), ".service") || strings.Contains(filepath.ToSlash(p), "systemd") {
				name := path.Base(filepath.ToSlash(p))
				found[name] = SystemdFile{Name: name, SourcePath: p}
			}
			return nil
		})
	}

	var missing []string
	for name := range referenced {
		if _, ok := found[name]; !ok && !skipMissing {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("systemd units not found: %s", strings.Join(missing, ", "))
	}

	var out []SystemdFile
	for _, file := range found {
		out = append(out, file)
	}
	return out, nil
}

func lookupString(doc map[string]any, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	cur := doc
	for i, key := range keys {
		if i == len(keys)-1 {
			if val, ok := cur[key]; ok {
				if s, ok := val.(string); ok {
					return s
				}
			}
			return ""
		}
		next, ok := cur[key].(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}

// lookupStringList reads a YAML list of strings from a map key.
func lookupStringList(m map[string]any, key string) []string {
	val, ok := m[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	case string:
		var out []string
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// lookupInt reads an integer value from a map key.
func lookupInt(m map[string]any, key string) int {
	val, ok := m[key]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return 0
}

// lookupBool reads a boolean value from a map key.
func lookupBool(m map[string]any, key string) bool {
	val, ok := m[key]
	if !ok {
		return false
	}
	b, _ := val.(bool)
	return b
}

func lookupMap(doc map[string]any, key string) map[string]any {
	if val, ok := doc[key]; ok {
		if m, ok := val.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func execFromCommand(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "ExecStart=")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	first := strings.Trim(fields[0], "\"'\\")
	if first == "" {
		return ""
	}
	return path.Base(first)
}

func execFromBinString(value string) string {
	lowered := strings.ToLower(value)
	for _, marker := range []string{"/internal/assets/bin/", "/assets/bin/", "/usr/lib/globular/bin/"} {
		if strings.Contains(lowered, marker) {
			idx := strings.LastIndex(lowered, marker)
			if idx >= 0 {
				segment := value[idx+len(marker):]
				segment = strings.TrimSpace(segment)
				segment = strings.TrimLeft(segment, " /\\")
				parts := strings.FieldsFunc(segment, func(r rune) bool {
					return r == ' ' || r == '\\' || r == '/' || r == '"'
				})
				if len(parts) > 0 {
					return path.Base(parts[0])
				}
			}
		}
	}
	return ""
}

func collectStrings(node any) []string {
	var out []string
	switch v := node.(type) {
	case string:
		out = append(out, v)
	case []any:
		for _, elem := range v {
			out = append(out, collectStrings(elem)...)
		}
	case map[string]any:
		for _, elem := range v {
			out = append(out, collectStrings(elem)...)
		}
	}
	return out
}

func configKeyFromString(value string) string {
	lowered := strings.ToLower(value)
	for _, marker := range []string{"/internal/assets/config/", "/assets/config/"} {
		if strings.Contains(lowered, marker) {
			idx := strings.Index(lowered, marker)
			if idx >= 0 {
				remainder := value[idx+len(marker):]
				remainder = strings.TrimLeft(remainder, "/")
				parts := strings.Split(remainder, "/")
				if len(parts) > 0 {
					key := parts[0]
					key = strings.TrimSpace(key)
					key = strings.TrimSuffix(key, " ")
					key = strings.Trim(key, "\"'")
					return normalizeServiceName(key)
				}
			}
		}
	}
	return ""
}

func collectInlineUnits(doc map[string]any) map[string]string {
	results := make(map[string]string)
	walk(doc, func(node any) {
		arr, ok := node.([]any)
		if !ok {
			return
		}
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			content, _ := m["content"].(string)
			if strings.HasSuffix(name, ".service") && content != "" {
				results[name] = content
			}
		}
	})
	return results
}

func collectSystemdReferences(doc map[string]any) map[string]struct{} {
	refs := make(map[string]struct{})
	for _, s := range collectStrings(doc) {
		lowered := strings.ToLower(s)
		if strings.Contains(lowered, ".service") || strings.Contains(lowered, "systemd") || strings.Contains(lowered, "/etc/systemd") {
			name := path.Base(s)
			if strings.Contains(name, ".service") {
				refs[name] = struct{}{}
			}
		}
	}
	return refs
}

func walk(node any, fn func(any)) {
	fn(node)
	switch v := node.(type) {
	case []any:
		for _, elem := range v {
			walk(elem, fn)
		}
	case map[string]any:
		for _, elem := range v {
			walk(elem, fn)
		}
	}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// discoverScripts finds .sh files in ScriptsRoot/<serviceName>/.
func discoverScripts(roots AssetRoots, serviceName string) []ScriptFile {
	if roots.ScriptsRoot == "" {
		return nil
	}
	candidates := []string{
		filepath.Join(roots.ScriptsRoot, serviceName),
		filepath.Join(roots.ScriptsRoot, strings.ReplaceAll(serviceName, "-", "_")),
	}
	seen := make(map[string]struct{})
	var scripts []ScriptFile
	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
				continue
			}
			if _, ok := seen[e.Name()]; ok {
				continue
			}
			seen[e.Name()] = struct{}{}
			scripts = append(scripts, ScriptFile{
				Name:       e.Name(),
				SourcePath: filepath.Join(dir, e.Name()),
			})
		}
	}
	return scripts
}

func uniquePathsWithSystemd(paths []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "/systemd") || strings.HasSuffix(p, "\\systemd") {
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				out = append(out, p)
			}
			continue
		}
		// allow raw config dir; append systemd child
		candidate := filepath.Join(p, "systemd")
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

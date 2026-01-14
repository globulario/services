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

// SpecInfo contains derived data from a spec.
type SpecInfo struct {
	SpecPath    string
	SpecFile    string
	ServiceName string
	ExecName    string
	ExecPath    string
	ConfigDirs  []string
	Systemd     []SystemdFile
}

type SystemdFile struct {
	Name       string // relative path within systemd/
	SourcePath string // optional: copy from this path
	Content    []byte // optional: inline content
}

type AssetRoots struct {
	BinRoot    string
	ConfigRoot string
}

type ScanOptions struct {
	SkipMissingConfig  bool
	SkipMissingSystemd bool
}

// ScanSpec reads a spec file and derives service metadata, executable, config, and systemd assets.
func ScanSpec(specPath string, roots AssetRoots, opts ScanOptions) (*SpecInfo, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	serviceName := deriveServiceName(specPath, doc)
	execName, err := deriveExecName(doc, roots, serviceName)
	if err != nil {
		return nil, fmt.Errorf("spec %s: %w", specPath, err)
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

	return &SpecInfo{
		SpecPath:    specPath,
		SpecFile:    filepath.Base(specPath),
		ServiceName: serviceName,
		ExecName:    execName,
		ExecPath:    execPath,
		ConfigDirs:  configDirs,
		Systemd:     systemdFiles,
	}, nil
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

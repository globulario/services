package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/globulario/services/golang/awareness/scan"
	"gopkg.in/yaml.v3"
)

// violationPattern is a code smell that awareness can detect.
type violationPattern struct {
	ID              string
	Pattern         *regexp.Regexp
	KnowledgeID     string // invariant or forbidden_fix ID
	Severity        string
	WhyDangerous    string
	SafeAlternative string
	// FileFilter: if non-empty, only flag when the file path matches.
	FileFilter *regexp.Regexp
	// FileExclude: skip files matching this (e.g. test files, this scanner itself).
	FileExclude *regexp.Regexp
}

var codeViolationPatterns = []violationPattern{
	{
		ID:              "localhost_interservice",
		Pattern:         regexp.MustCompile(`"127\.0\.0\.1|"localhost`),
		KnowledgeID:     "service.endpoint.no_localhost_interservice",
		Severity:        "critical",
		WhyDangerous:    "Loopback addresses break multi-node cluster semantics and hide endpoint ownership bugs.",
		SafeAlternative: "Resolve the address from etcd or use the controller's service discovery.",
		FileExclude:     regexp.MustCompile(`_test\.go|awareness`),
	},
	{
		ID:              "os_exec_in_controller",
		Pattern:         regexp.MustCompile(`"os/exec"`),
		KnowledgeID:     "service.security.no_exec_in_controller",
		Severity:        "critical",
		WhyDangerous:    "os/exec in cluster_controller violates the security boundary — only node_agent may spawn processes.",
		SafeAlternative: "Move execution logic to a workflow step dispatched to node_agent.",
		FileFilter:      regexp.MustCompile(`cluster_controller`),
		FileExclude:     regexp.MustCompile(`_test\.go`),
	},
	{
		ID:              "os_getenv_config",
		Pattern:         regexp.MustCompile(`os\.Getenv\(`),
		KnowledgeID:     "config.no_env_var_authority",
		Severity:        "high",
		WhyDangerous:    "os.Getenv for service config violates the etcd-only authority rule.",
		SafeAlternative: "Read configuration from etcd via the config package.",
		FileExclude:     regexp.MustCompile(`_test\.go|globularcli|generateCode|build-all`),
	},
	{
		ID:              "hardcoded_grpc_port",
		Pattern:         regexp.MustCompile(`:\d{5}"|\bport\s*:=\s*\d{5}\b`),
		KnowledgeID:     "config.no_hardcoded_service_ports",
		Severity:        "high",
		WhyDangerous:    "Hardcoded gRPC service ports bypass etcd-based service discovery.",
		SafeAlternative: "Resolve port from etcd at runtime.",
		FileExclude:     regexp.MustCompile(`_test\.go|awareness|proto`),
	},
	{
		ID:              "restart_without_pkill",
		Pattern:         regexp.MustCompile(`systemctl\s+restart|Restart=on-failure`),
		KnowledgeID:     "service.endpoint.port_squatting_cgroup_escape",
		Severity:        "medium",
		WhyDangerous:    "Restart without killing orphaned processes can cause port squatting (cgroup escape).",
		SafeAlternative: "Use ExecStartPre=+/bin/sh -c 'pkill -9 -f <binary> || true' in the unit file.",
		FileExclude:     regexp.MustCompile(`_test\.go|awareness`),
	},
	{
		ID:              "minio_local_inference",
		Pattern:         regexp.MustCompile(`filepath\.Walk|ioutil\.ReadDir|os\.ReadDir`),
		KnowledgeID:     "objectstore.local_membership_inference",
		Severity:        "medium",
		WhyDangerous:    "Inferring MinIO topology from local disk instead of ObjectStoreDesiredState in etcd leads to split-brain.",
		SafeAlternative: "Read MinIO topology from etcd ObjectStoreDesiredState key.",
		FileFilter:      regexp.MustCompile(`minio|objectstore|storage`),
		FileExclude:     regexp.MustCompile(`_test\.go|awareness`),
	},
	{
		ID:              "direct_state_write",
		Pattern:         regexp.MustCompile(`etcd\.Put|clientv3\..*Put\(`),
		KnowledgeID:     "config.no_direct_etcd_state_write",
		Severity:        "high",
		WhyDangerous:    "Direct etcd writes bypass the controller authority model and can create split-brain state.",
		SafeAlternative: "Use the controller gRPC API or workflow steps for state mutations.",
		FileExclude:     regexp.MustCompile(`_test\.go|awareness|config/etcd`),
	},
	{
		ID:              "blind_retry_loop",
		Pattern:         regexp.MustCompile(`for\s*{|for\s+err\s*!=\s*nil|goto\s+retry`),
		KnowledgeID:     "deterministic.install.failure.retry_loop",
		Severity:        "medium",
		WhyDangerous:    "Blind retry loops without terminal classification can spin forever on deterministic failures.",
		SafeAlternative: "Use FailureClass classification and workflow retry policy with max attempts.",
		FileExclude:     regexp.MustCompile(`_test\.go|awareness`),
	},
	{
		ID:              "insecure_grpc_transport",
		Pattern:         regexp.MustCompile(`insecure\.NewCredentials\(\)|grpc\.WithInsecure\(\)`),
		KnowledgeID:     "service.security.no_insecure_grpc_interservice",
		Severity:        "critical",
		WhyDangerous:    "Insecure gRPC transport exposes cluster RPCs to interception on the network.",
		SafeAlternative: "Use mTLS with the cluster CA cert and service identity certs.",
		FileExclude:     regexp.MustCompile(`_test\.go|awareness/runtime/grpc_|awareness/runtime/prometheus`),
	},
}

type violationFinding struct {
	File            string `json:"file"`
	Line            int    `json:"line"`
	Column          int    `json:"column"`       // byte offset of match start in line
	Snippet         string `json:"snippet"`
	PatternID       string `json:"pattern_id"`
	KnowledgeID     string `json:"knowledge_id"`
	Severity        string `json:"severity"`
	WhyDangerous    string `json:"why_dangerous"`
	SafeAlternative string `json:"safe_alternative"`
	Confidence      string `json:"confidence"` // "high" for regex match; future: "medium" for heuristic
}

// scanAllowlistEntry is a single entry in the scan allowlist.
type scanAllowlistEntry struct {
	PathPattern string `yaml:"path_pattern"`
	PatternID   string `yaml:"pattern_id"`
	Reason      string `yaml:"reason"`
}

type scanAllowlist struct {
	Allowlist []scanAllowlistEntry `yaml:"allowlist"`
}

func loadScanAllowlist(docsDir string) []scanAllowlistEntry {
	if docsDir == "" {
		return nil
	}
	path := filepath.Join(docsDir, "knowledge", "scan_allowlist.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var al scanAllowlist
	if err := yaml.Unmarshal(data, &al); err != nil {
		return nil
	}
	return al.Allowlist
}

// matchesAllowlist returns the allowlist entry that suppresses this finding, or nil.
func matchesAllowlist(f violationFinding, allowlist []scanAllowlistEntry) *scanAllowlistEntry {
	for i, entry := range allowlist {
		if entry.PatternID != f.PatternID {
			continue
		}
		if matchesPathPattern(f.File, entry.PathPattern) {
			return &allowlist[i]
		}
	}
	return nil
}

// matchesPathPattern checks whether filePath matches a glob pattern.
// Supports:
//   - "**/*.ext" — any file ending with the extension in any directory
//   - "glob/pattern" — filepath.Match against base name or full path
func matchesPathPattern(filePath, pattern string) bool {
	if strings.HasPrefix(pattern, "**/") {
		// Strip "**/" and match the rest as a glob against the base name.
		rest := pattern[3:]
		base := filepath.Base(filePath)
		matched, err := filepath.Match(rest, base)
		if err == nil && matched {
			return true
		}
		// Also check if any path component matches.
		return false
	}
	// Try base name match.
	matched, err := filepath.Match(pattern, filepath.Base(filePath))
	if err == nil && matched {
		return true
	}
	// Try full path match.
	matched, err = filepath.Match(pattern, filePath)
	if err == nil && matched {
		return true
	}
	return false
}

func registerScanViolationsTool(s *server, st *awarenessState) {
	// Load allowlist once at registration time.
	allowlist := loadScanAllowlist(st.docsDir)

	s.register(toolDef{
		Name:        "awareness.scan_violations",
		Description: "Scan Go source files for forbidden architecture patterns. Each finding maps back to a known failure mode, forbidden fix, or invariant.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"root":          {Type: "string", Description: "Root directory to scan. Defaults to repo root."},
				"paths":         {Type: "array", Description: "Optional list of sub-paths to restrict the scan.", Items: &propSchema{Type: "string"}},
				"severity":      {Type: "string", Description: "Minimum severity to report: critical, high, medium (default: medium).", Enum: []string{"critical", "high", "medium"}},
				"include_tests": {Type: "boolean", Description: "Include _test.go files (default false)."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx
		root := strArg(args, "root")
		if root == "" {
			root = st.repoRoot
		}
		if root == "" {
			return nil, fmt.Errorf("root directory not configured")
		}
		paths := strSliceArg(args, "paths")
		if len(paths) == 0 {
			paths = []string{filepath.Join(root, "golang")}
		} else {
			for i, p := range paths {
				if !filepath.IsAbs(p) {
					paths[i] = filepath.Join(root, p)
				}
			}
		}
		minSeverity := strArg(args, "severity")
		if minSeverity == "" {
			minSeverity = "medium"
		}
		includeTests := boolArg(args, "include_tests")

		var allFindings []violationFinding
		for _, scanPath := range paths {
			found, err := scanPathForViolations(scanPath, minSeverity, includeTests)
			if err != nil {
				return nil, fmt.Errorf("scan %s: %w", scanPath, err)
			}
			allFindings = append(allFindings, found...)
		}

		// Deduplicate: when regex and AST both fire on the same (file, line),
		// prefer the regex finding (it's already allowlist-aware by patternID).
		allFindings = deduplicateViolationFindings(allFindings)

		// Apply allowlist. Track suppressed (file, line) positions so that AST
		// findings at the same location are also suppressed even when the AST
		// patternID differs from the allowlist entry's patternID.
		type fileLine struct{ file string; line int }
		suppressedLocs := make(map[fileLine]bool)
		var findings []violationFinding
		var suppressed []violationFinding
		for _, f := range allFindings {
			loc := fileLine{f.File, f.Line}
			if suppressedLocs[loc] {
				suppressed = append(suppressed, f)
				continue
			}
			if entry := matchesAllowlist(f, allowlist); entry != nil {
				suppressed = append(suppressed, f)
				suppressedLocs[loc] = true
			} else {
				findings = append(findings, f)
			}
		}

		// Summarise by knowledge ID.
		byKnowledge := make(map[string]int)
		for _, f := range findings {
			byKnowledge[f.KnowledgeID]++
		}

		return map[string]interface{}{
			"findings":            findings,
			"suppressed_findings": suppressed,
			"total":               len(findings),
			"suppressed_count":    len(suppressed),
			"by_knowledge_id":     byKnowledge,
			"scanned_paths":       paths,
		}, nil
	})
}

func scanPathForViolations(root, minSeverity string, includeTests bool) ([]violationFinding, error) {
	var findings []violationFinding
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.IsDir() {
			// Skip vendor, .git, generated protobuf dirs.
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || strings.HasSuffix(base, "pb") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if !includeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found := scanFileForViolations(path, minSeverity)
		findings = append(findings, found...)

		// Also run AST scanner and merge results.
		astFindings, _ := scan.ScanGoFile(path, nil)
		for _, af := range astFindings {
			if !severityAtLeast(af.Severity, minSeverity) {
				continue
			}
			findings = append(findings, violationFinding{
				File:            af.File,
				Line:            af.Line,
				Column:          af.Column,
				Snippet:         af.Snippet,
				PatternID:       af.PatternID,
				KnowledgeID:     af.KnowledgeID,
				Severity:        af.Severity,
				WhyDangerous:    af.WhyDangerous,
				SafeAlternative: af.SafeAlternative,
				Confidence:      af.Confidence,
			})
		}
		return nil
	})
	return findings, err
}

func scanFileForViolations(path, minSeverity string) []violationFinding {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var findings []violationFinding
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, pat := range codeViolationPatterns {
			if !severityAtLeast(pat.Severity, minSeverity) {
				continue
			}
			if pat.FileFilter != nil && !pat.FileFilter.MatchString(path) {
				continue
			}
			if pat.FileExclude != nil && pat.FileExclude.MatchString(path) {
				continue
			}
			loc := pat.Pattern.FindStringIndex(line)
			if loc == nil {
				continue
			}
			col := loc[0]
			findings = append(findings, violationFinding{
				File:            path,
				Line:            lineNum,
				Column:          col,
				Snippet:         strings.TrimSpace(line),
				PatternID:       pat.ID,
				KnowledgeID:     pat.KnowledgeID,
				Severity:        pat.Severity,
				WhyDangerous:    pat.WhyDangerous,
				SafeAlternative: pat.SafeAlternative,
				Confidence:      "high",
			})
		}
	}
	return findings
}

func severityAtLeast(have, min string) bool {
	order := map[string]int{"medium": 1, "high": 2, "critical": 3, "warning": 1}
	return order[have] >= order[min]
}

// deduplicateViolationFindings removes duplicate findings at the same (file, line).
// Regex findings are preferred over AST findings for the same location, since
// regex findings are allowlist-registered by patternID.
// Among same-location findings, the first one seen is kept (regex comes first).
func deduplicateViolationFindings(findings []violationFinding) []violationFinding {
	type loc struct {
		file string
		line int
	}
	seen := make(map[loc]bool)
	out := make([]violationFinding, 0, len(findings))
	for _, f := range findings {
		k := loc{f.File, f.Line}
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	return out
}

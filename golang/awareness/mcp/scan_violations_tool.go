package mcp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
}

type violationFinding struct {
	File            string `json:"file"`
	Line            int    `json:"line"`
	Snippet         string `json:"snippet"`
	PatternID       string `json:"pattern_id"`
	KnowledgeID     string `json:"knowledge_id"`
	Severity        string `json:"severity"`
	WhyDangerous    string `json:"why_dangerous"`
	SafeAlternative string `json:"safe_alternative"`
}

func registerScanViolationsTool(s *Server) {
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
			root = s.resolvedRepoRoot()
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

		var findings []violationFinding
		for _, scanPath := range paths {
			found, err := scanPathForViolations(scanPath, minSeverity, includeTests)
			if err != nil {
				return nil, fmt.Errorf("scan %s: %w", scanPath, err)
			}
			findings = append(findings, found...)
		}

		// Summarise by knowledge ID.
		byKnowledge := make(map[string]int)
		for _, f := range findings {
			byKnowledge[f.KnowledgeID]++
		}

		return map[string]interface{}{
			"findings":        findings,
			"total":           len(findings),
			"by_knowledge_id": byKnowledge,
			"scanned_paths":   paths,
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
			if pat.Pattern.MatchString(line) {
				findings = append(findings, violationFinding{
					File:            path,
					Line:            lineNum,
					Snippet:         strings.TrimSpace(line),
					PatternID:       pat.ID,
					KnowledgeID:     pat.KnowledgeID,
					Severity:        pat.Severity,
					WhyDangerous:    pat.WhyDangerous,
					SafeAlternative: pat.SafeAlternative,
				})
			}
		}
	}
	return findings
}

func severityAtLeast(have, min string) bool {
	order := map[string]int{"medium": 1, "high": 2, "critical": 3}
	return order[have] >= order[min]
}

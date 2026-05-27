package intentaudit

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Status constants for audit results.
const (
	StatusPass               = "PASS"
	StatusCandidateViolation = "CANDIDATE_VIOLATION"
	StatusAcceptedException  = "ACCEPTED_EXCEPTION"
	StatusTestCoverageGap    = "TEST_COVERAGE_GAP"
	StatusMissingTest        = "MISSING_REQUIRED_TEST"
)

// Finding is a single audit observation for one intent node.
type Finding struct {
	IntentID    string `json:"intent_id" yaml:"intent_id"`
	Status      string `json:"status" yaml:"status"`
	Pattern     string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	File        string `json:"file,omitempty" yaml:"file,omitempty"`
	Line        int    `json:"line,omitempty" yaml:"line,omitempty"`
	ExceptionID string `json:"exception_id,omitempty" yaml:"exception_id,omitempty"`
	Detail      string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// IntentResult is the aggregated audit result for one intent node.
type IntentResult struct {
	IntentID string    `json:"intent_id" yaml:"intent_id"`
	Title    string    `json:"title" yaml:"title"`
	Status   string    `json:"status" yaml:"status"` // worst status across findings
	Findings []Finding `json:"findings" yaml:"findings"`
}

// AuditReport is the full output of an audit run.
type AuditReport struct {
	RunID           string          `json:"run_id" yaml:"run_id"`
	Timestamp       time.Time       `json:"timestamp" yaml:"timestamp"`
	ChangedFiles    []string        `json:"changed_files,omitempty" yaml:"changed_files,omitempty"`
	Results         []IntentResult  `json:"intent_results" yaml:"intent_results"`
	ASTFindings     []ASTFinding    `json:"ast_findings,omitempty" yaml:"ast_findings,omitempty"`
	RuntimeEvidence []RuntimeResult `json:"runtime_evidence,omitempty" yaml:"runtime_evidence,omitempty"`
	Summary         AuditSummary    `json:"summary" yaml:"summary"`
}

// AuditSummary counts results by status.
type AuditSummary struct {
	Total              int `json:"total" yaml:"total"`
	Pass               int `json:"pass" yaml:"pass"`
	CandidateViolation int `json:"candidate_violation" yaml:"candidate_violation"`
	AcceptedException  int `json:"accepted_exception" yaml:"accepted_exception"`
	TestCoverageGap    int `json:"test_coverage_gap" yaml:"test_coverage_gap"`
	MissingTest        int `json:"missing_test" yaml:"missing_test"`
}

// AuditOptions configures an audit run.
type AuditOptions struct {
	IntentDir string   // path to docs/intent/
	SrcDir    string   // Go source root for violation pattern scanning
	ScopeIDs  []string // if non-empty, only audit these intent IDs
}

// RunAudit performs a full intent audit: test checks + violation scans.
func RunAudit(opts AuditOptions) (*AuditReport, error) {
	nodes, loadErrs := LoadDir(opts.IntentDir)
	if len(nodes) == 0 && len(loadErrs) > 0 {
		return nil, loadErrs[0]
	}

	report := &AuditReport{
		RunID:     fmt.Sprintf("audit-%d", time.Now().Unix()),
		Timestamp: time.Now(),
	}

	for _, node := range sortedNodes(nodes) {
		if len(opts.ScopeIDs) > 0 && !contains(opts.ScopeIDs, node.ID) {
			continue
		}
		result := auditNode(node, opts.SrcDir)
		report.Results = append(report.Results, result)
	}

	// AST-based precision scanning (supplements grep-based violation patterns).
	if opts.SrcDir != "" {
		allExceptions := collectAllExceptions(nodes)
		astFindings, _ := ScanGoAST(opts.SrcDir, allExceptions)
		report.ASTFindings = astFindings
	}

	report.Summary = summarize(report.Results)
	return report, nil
}

func auditNode(node *Node, srcDir string) IntentResult {
	result := IntentResult{
		IntentID: node.ID,
		Title:    node.Title,
		Status:   StatusPass,
	}

	// Phase 1: required_tests check.
	testFindings := checkRequiredTests(node, srcDir)
	result.Findings = append(result.Findings, testFindings...)

	// Phase 2: violation_patterns scan.
	if srcDir != "" && len(node.ViolationPatterns) > 0 {
		violationFindings := scanViolationPatterns(node, srcDir)
		result.Findings = append(result.Findings, violationFindings...)
	}

	// Determine worst status.
	result.Status = worstStatus(result.Findings)
	if result.Status == "" {
		result.Status = StatusPass
	}
	return result
}

// checkRequiredTests verifies that each listed test exists in the source.
func checkRequiredTests(node *Node, srcDir string) []Finding {
	if len(node.RequiredTests) == 0 {
		// No tests listed — check if this is a known gap.
		return []Finding{{
			IntentID: node.ID,
			Status:   StatusTestCoverageGap,
			Detail:   "no required_tests listed",
		}}
	}

	var findings []Finding
	for _, testRef := range node.RequiredTests {
		testName := extractTestName(testRef)
		if testName == "" {
			continue
		}
		if srcDir != "" && testExistsInSource(testName, srcDir) {
			findings = append(findings, Finding{
				IntentID: node.ID,
				Status:   StatusPass,
				Detail:   fmt.Sprintf("test %s exists", testName),
			})
		} else {
			findings = append(findings, Finding{
				IntentID: node.ID,
				Status:   StatusMissingTest,
				Detail:   fmt.Sprintf("test %s not found", testName),
			})
		}
	}
	return findings
}

// scanViolationPatterns greps source code for patterns that would violate
// the intent. Matches under named exceptions are reported as ACCEPTED_EXCEPTION.
func scanViolationPatterns(node *Node, srcDir string) []Finding {
	var findings []Finding
	for _, pattern := range node.ViolationPatterns {
		matches := grepPattern(pattern, srcDir)
		for _, m := range matches {
			excID := matchesException(m.file, m.line, node.Exceptions)
			if excID != "" {
				findings = append(findings, Finding{
					IntentID:    node.ID,
					Status:      StatusAcceptedException,
					Pattern:     pattern,
					File:        m.file,
					Line:        m.line,
					ExceptionID: excID,
				})
			} else {
				findings = append(findings, Finding{
					IntentID: node.ID,
					Status:   StatusCandidateViolation,
					Pattern:  pattern,
					File:     m.file,
					Line:     m.line,
				})
			}
		}
	}
	return findings
}

type grepMatch struct {
	file string
	line int
}

// grepPattern runs rg (or grep) for a pattern in Go files under srcDir.
func grepPattern(pattern, srcDir string) []grepMatch {
	// Try rg first, fall back to grep.
	args := []string{"-rn", "--type", "go", "--no-heading", pattern, srcDir}
	cmd := exec.Command("rg", args...)
	out, err := cmd.Output()
	if err != nil {
		// Fall back to grep.
		args = []string{"-rn", "--include=*.go", pattern, srcDir}
		cmd = exec.Command("grep", args...)
		out, _ = cmd.Output()
	}

	var matches []grepMatch
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// Skip test files.
		if strings.Contains(line, "_test.go:") {
			continue
		}
		// Skip files that define violation scanning rules — they
		// naturally contain violation pattern strings as rule definitions,
		// not actual violations.
		if strings.Contains(line, "scan_violations") ||
			strings.Contains(line, "self_review") ||
			strings.Contains(line, "violation_patterns") {
			continue
		}
		file, lineNo := parseGrepLine(line, srcDir)
		if file != "" {
			matches = append(matches, grepMatch{file: file, line: lineNo})
		}
	}
	return matches
}

func parseGrepLine(line, srcDir string) (string, int) {
	// Format: filepath:linenum:content
	parts := strings.SplitN(line, ":", 3)
	if len(parts) < 2 {
		return "", 0
	}
	file := parts[0]
	// Make path relative to srcDir for readability.
	if rel, err := filepath.Rel(srcDir, file); err == nil {
		file = rel
	}
	lineNo := 0
	fmt.Sscanf(parts[1], "%d", &lineNo)
	return file, lineNo
}

// testExistsInSource checks if a test function name exists in any _test.go file.
func testExistsInSource(testName, srcDir string) bool {
	args := []string{"-rn", "--type", "go", "--glob", "*_test.go",
		fmt.Sprintf("func %s\\b", testName), srcDir}
	cmd := exec.Command("rg", args...)
	out, err := cmd.Output()
	if err != nil {
		// Fall back to grep.
		args = []string{"-rn", "--include=*_test.go",
			fmt.Sprintf("func %s", testName), srcDir}
		cmd = exec.Command("grep", args...)
		out, _ = cmd.Output()
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// matchesException returns the exception ID if the file/line match falls
// under a named exception. Checks explicit file patterns first, then falls
// back to keyword matching in the description against the filename.
func matchesException(file string, line int, exceptions []Exception) string {
	_ = line // future: line-range matching
	fileLower := strings.ToLower(file)
	for _, exc := range exceptions {
		// Check explicit file patterns first.
		for _, fp := range exc.Files {
			if strings.Contains(fileLower, strings.ToLower(fp)) {
				return exc.ExceptionID()
			}
		}
		// Fall back to keyword matching in description.
		desc := strings.ToLower(exc.Description)
		if strings.Contains(desc, "bootstrap") && strings.Contains(fileLower, "bootstrap") {
			return exc.ExceptionID()
		}
		if strings.Contains(desc, "probe") && strings.Contains(fileLower, "probe") {
			return exc.ExceptionID()
		}
		if strings.Contains(desc, "loopback") && (strings.Contains(fileLower, "repository") ||
			strings.Contains(fileLower, "media") || strings.Contains(fileLower, "file_server")) {
			return exc.ExceptionID()
		}
		if strings.Contains(desc, "reachability") && strings.Contains(fileLower, "clients") {
			return exc.ExceptionID()
		}
		if strings.Contains(desc, "convergence_committer") && strings.Contains(fileLower, "convergence_committer") {
			return exc.ExceptionID()
		}
		if strings.Contains(desc, "healer") && strings.Contains(fileLower, "healer") {
			return exc.ExceptionID()
		}
		if strings.Contains(desc, "remediation") && strings.Contains(fileLower, "remediation") {
			return exc.ExceptionID()
		}
	}
	return ""
}

// extractTestName gets the test function name from a "pkg:TestName" reference.
func extractTestName(ref string) string {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func sortedNodes(nodes map[string]*Node) []*Node {
	keys := make([]string, 0, len(nodes))
	for k := range nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sorted := make([]*Node, len(keys))
	for i, k := range keys {
		sorted[i] = nodes[k]
	}
	return sorted
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func worstStatus(findings []Finding) string {
	priority := map[string]int{
		StatusCandidateViolation: 5,
		StatusMissingTest:        4,
		StatusTestCoverageGap:    3,
		StatusAcceptedException:  2,
		StatusPass:               1,
	}
	worst := ""
	worstPri := 0
	for _, f := range findings {
		if p := priority[f.Status]; p > worstPri {
			worstPri = p
			worst = f.Status
		}
	}
	return worst
}

func summarize(results []IntentResult) AuditSummary {
	s := AuditSummary{Total: len(results)}
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			s.Pass++
		case StatusCandidateViolation:
			s.CandidateViolation++
		case StatusAcceptedException:
			s.AcceptedException++
		case StatusTestCoverageGap:
			s.TestCoverageGap++
		case StatusMissingTest:
			s.MissingTest++
		}
	}
	return s
}

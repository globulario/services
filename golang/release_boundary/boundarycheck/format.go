package boundarycheck

import (
	"fmt"
	"sort"
	"strings"

	"github.com/globulario/services/golang/release_boundary"
)

// ExitCode returns the process exit code for a verdict: 0 only for PROVEN, 2
// for every other outcome (FAILED / INDETERMINATE / NOT_APPLICABLE). Callers
// should also treat collection errors as a non-zero condition.
func ExitCode(v release_boundary.Verdict) int {
	if v == release_boundary.VerdictProven {
		return 0
	}
	return 2
}

// FormatReport renders a human-readable release-boundary report, including the
// per-assertion verdicts/reasons, provenance, and any collection errors.
func FormatReport(r release_boundary.Report, ev *Evidence) string {
	var b strings.Builder

	fmt.Fprintf(&b, "release boundary: %s on %s\n", r.ServiceName, r.NodeName)
	fmt.Fprintf(&b, "  verdict:   %s\n", r.Verdict)
	if r.BuildID != "" {
		fmt.Fprintf(&b, "  build_id:  %s\n", r.BuildID)
	}
	if r.Checksum != "" {
		fmt.Fprintf(&b, "  checksum:  %s\n", r.Checksum)
	}
	if gitSHA := provenanceGitSHA(ev); gitSHA != "" {
		fmt.Fprintf(&b, "  provenance git_sha: %s\n", gitSHA)
	}

	b.WriteString("  assertions:\n")
	for _, a := range r.Assertions {
		fmt.Fprintf(&b, "    %-2s %-13s %-26s %s\n", a.ID, a.Verdict, a.Name, a.Reason)
	}

	if ev != nil && len(ev.CollectionErrors) > 0 {
		b.WriteString("  collection_errors:\n")
		for _, k := range sortedKeys(ev.CollectionErrors) {
			fmt.Fprintf(&b, "    %-10s %s\n", k+":", ev.CollectionErrors[k])
		}
	}
	return b.String()
}

// ReportToMap serializes a report for a JSON/MCP envelope. Collection errors,
// when present, are attached under "collection_errors".
func ReportToMap(r release_boundary.Report, ev *Evidence) map[string]interface{} {
	assertions := make([]map[string]interface{}, 0, len(r.Assertions))
	for _, a := range r.Assertions {
		assertions = append(assertions, map[string]interface{}{
			"id":       string(a.ID),
			"name":     a.Name,
			"verdict":  string(a.Verdict),
			"reason":   a.Reason,
			"evidence": a.Evidence,
		})
	}
	out := map[string]interface{}{
		"service":    r.ServiceName,
		"node":       r.NodeName,
		"build_id":   r.BuildID,
		"checksum":   r.Checksum,
		"verdict":    string(r.Verdict),
		"assertions": assertions,
	}
	if gitSHA := provenanceGitSHA(ev); gitSHA != "" {
		out["provenance_git_sha"] = gitSHA
	}
	if ev != nil && len(ev.CollectionErrors) > 0 {
		out["collection_errors"] = ev.CollectionErrors
	}
	return out
}

func provenanceGitSHA(ev *Evidence) string {
	if ev == nil || ev.Manifest == nil {
		return ""
	}
	return ev.Manifest.GetProvenance().GetBuildCommit()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

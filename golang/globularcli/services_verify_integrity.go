package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/spf13/cobra"
)

// services verify-integrity — runs the package.verify_integrity action on a
// node via the new VerifyPackageIntegrity gRPC method and prints findings.
//
// Design choices:
//   - Calls a node_agent directly (not the cluster controller). Operators
//     typically want a per-node view; the doctor rule runs cluster-wide.
//   - Accepts --package to narrow to one package, --kind to narrow by kind.
//   - Prints JSON by default (the raw report from the action) so downstream
//     tooling can parse it. Table output is the default and shows findings
//     grouped by invariant.
//   - Exit code reflects finding severity: 0 if clean, 1 if any WARN/INFO,
//     2 if any ERROR.

var (
	svcVerifyPackage   string
	svcVerifyKind      string
	svcVerifyRepoAddr  string
	svcVerifyJSON      bool
	svcVerifyQuiet     bool
)

var servicesVerifyIntegrityCmd = &cobra.Command{
	Use:   "verify-integrity",
	Short: "Check installed packages against the repository manifest (read-only)",
	Long: `Runs the package.verify_integrity action on a node_agent and prints findings.

Verifies four invariants per installed package:
  * artifact.installed_digest_mismatch — installed sha256 vs manifest digest (ERROR)
  * artifact.desired_build_mismatch    — installed build vs desired build   (WARN)
  * artifact.cache_digest_mismatch     — cached tgz sha256 vs manifest      (WARN)
  * artifact.cache_missing             — resolved digest but no cache file  (INFO)

Exit codes:
  0  no findings
  1  at least one WARN or INFO finding
  2  at least one ERROR finding
  3  RPC or local invocation error

Examples:
  # Check all packages on the local node
  globular services verify-integrity

  # Check a specific package
  globular services verify-integrity --package event

  # Target a specific node (RPC into a remote node-agent)
  globular services verify-integrity --node 10.0.0.20:11000 --json
`,
	RunE: runServicesVerifyIntegrity,
}

func runServicesVerifyIntegrity(cmd *cobra.Command, args []string) error {
	nodeAddr := rootCfg.nodeAddr
	cc, err := dialGRPC(nodeAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial %s: %v\n", nodeAddr, err)
		os.Exit(3)
	}
	defer cc.Close()
	client := node_agentpb.NewNodeAgentServiceClient(cc)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.VerifyPackageIntegrity(ctx, &node_agentpb.VerifyPackageIntegrityRequest{
		PackageName:    svcVerifyPackage,
		Kind:           strings.ToUpper(svcVerifyKind),
		RepositoryAddr: svcVerifyRepoAddr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "VerifyPackageIntegrity: %v\n", err)
		os.Exit(3)
	}
	if !resp.GetOk() {
		fmt.Fprintf(os.Stderr, "verify-integrity failed: %s\n", resp.GetErrorDetail())
		os.Exit(3)
	}

	if svcVerifyJSON {
		// Pretty-print the JSON. The action already indents; just output.
		fmt.Println(resp.GetReportJson())
	} else {
		printVerifyIntegrityTable(resp)
	}

	// Determine exit code from finding severities.
	sev := worstSeverity(resp.GetReportJson())
	switch sev {
	case "ERROR":
		os.Exit(2)
	case "WARN", "INFO":
		os.Exit(1)
	default:
		return nil
	}
	return nil
}

// printVerifyIntegrityTable renders a concise human-readable summary.
func printVerifyIntegrityTable(resp *node_agentpb.VerifyPackageIntegrityResponse) {
	var report struct {
		NodeID     string         `json:"node_id"`
		Checked    int            `json:"checked"`
		Invariants map[string]int `json:"invariants"`
		Errors     []string       `json:"errors"`
		Findings   []struct {
			Invariant string            `json:"invariant"`
			Severity  string            `json:"severity"`
			Package   string            `json:"package"`
			Kind      string            `json:"kind"`
			Summary   string            `json:"summary"`
			Evidence  map[string]string `json:"evidence"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(resp.GetReportJson()), &report); err != nil {
		fmt.Fprintf(os.Stderr, "parse report: %v\n", err)
		fmt.Println(resp.GetReportJson())
		return
	}

	fmt.Printf("Node:     %s\n", report.NodeID)
	fmt.Printf("Checked:  %d packages\n", report.Checked)
	fmt.Printf("Findings: %d (%s)\n", resp.GetFindingCount(), summarizeInvariants(report.Invariants))
	if len(report.Errors) > 0 {
		fmt.Printf("Errors:   %d upstream errors during collection\n", len(report.Errors))
		if !svcVerifyQuiet {
			for _, e := range report.Errors {
				fmt.Printf("  - %s\n", e)
			}
		}
	}
	if len(report.Findings) == 0 {
		fmt.Println("\n✓ all checked packages match repository manifests")
		return
	}
	fmt.Println()
	fmt.Println("Findings:")
	for _, f := range report.Findings {
		mark := "⚠"
		if f.Severity == "ERROR" {
			mark = "✖"
		} else if f.Severity == "INFO" {
			mark = "·"
		}
		fmt.Printf("  %s [%s] %s/%s — %s\n", mark, f.Severity, f.Kind, f.Package, f.Summary)
		if svcVerifyQuiet {
			continue
		}
		for k, v := range f.Evidence {
			// Truncate long digests for readability.
			if len(v) > 20 && isHexish(v) {
				v = v[:16] + "…"
			}
			fmt.Printf("       %s = %s\n", k, v)
		}
	}
}

// summarizeInvariants formats the per-invariant counts inline.
func summarizeInvariants(m map[string]int) string {
	if len(m) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%d", k, v))
	}
	return strings.Join(parts, ", ")
}

// worstSeverity returns the highest severity found in the JSON report,
// or "" if the report has no findings.
func worstSeverity(reportJSON string) string {
	var report struct {
		Findings []struct {
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		return ""
	}
	order := map[string]int{"INFO": 1, "WARN": 2, "ERROR": 3}
	best := 0
	worst := ""
	for _, f := range report.Findings {
		if order[f.Severity] > best {
			best = order[f.Severity]
			worst = f.Severity
		}
	}
	return worst
}

// isHexish reports whether a string looks like a hex digest (length multiple
// of 2, only [0-9a-f]). Used to decide whether to truncate for display.
func isHexish(s string) bool {
	if s == "" || len(s)%2 != 0 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

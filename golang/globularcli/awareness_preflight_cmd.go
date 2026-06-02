package main

// awareness_preflight_cmd.go — CLI front-end for the Preflight RPC.
//
// Preflight composes Briefing's anchor matching with a typed risk
// classifier into a single bounded response. Intended for agents (and
// humans) to call BEFORE editing high-risk code: it returns one of six
// risk classes, a confidence tier, the required actions, and the
// forbidden fixes — all anchored to the graph, never invented.
//
// Usage:
//
//	globular awareness preflight --task "<text>" [--file <path> ...] [--mode compact|standard] [--json]
//
// Examples:
//
//	globular awareness preflight --task "wire a new service client" \
//	    --file golang/foo/foo_client/foo_client.go
//	globular awareness preflight --task "refactor reconcile loop" \
//	    --file golang/cluster_controller/reconciler.go --mode standard
//	globular awareness preflight --task "add a new RPC" --json

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
)

var (
	preflightTask  string
	preflightFiles []string
	preflightMode  string
	preflightJSON  bool
)

var awarenessPreflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Pre-edit decision support: risk class + required actions + forbidden fixes",
	Long: `Returns a structured pre-edit briefing tailored for an agent (or human)
about to write code:

  - risk_class        one of LOW_RISK | ARCHITECTURE_SENSITIVE |
                      CONVERGENCE_RISK | SECURITY_RISK | DATA_LOSS_RISK |
                      UNKNOWN_IMPACT
  - confidence        HIGH | MEDIUM | LOW
  - coverage          whether the graph has evidence for the request
  - required_actions  concrete things to do before editing
  - files_to_read     canonical references the matched pattern points at
  - tests_to_run      tests anchored to the touched files
  - forbidden_fixes   anchored forbid-edges + pattern's forbidden_calls
  - blind_spots       why the classifier chose this risk class; also
                      surfaces "coverage_insufficient" when the graph is
                      thin for this area

When the awareness-graph store is unavailable, the response is DEGRADED
with risk_class=UNKNOWN_IMPACT — proceed cautiously and re-run preflight
once the store is back.`,
	RunE: runAwarenessPreflight,
}

func runAwarenessPreflight(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(preflightTask) == "" && len(preflightFiles) == 0 {
		return fmt.Errorf("preflight needs at least one of --task or --file")
	}
	mode := parsePreflightMode(preflightMode)

	cli, err := awarenessDialClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := cli.Preflight(ctx, &awarenesspb.PreflightRequest{
		Task:  preflightTask,
		Files: preflightFiles,
		Mode:  mode,
	})
	if err != nil {
		return fmt.Errorf("preflight rpc: %w", err)
	}

	if preflightJSON {
		return emitPreflightJSON(resp)
	}
	printPreflightHuman(resp)
	return nil
}

func parsePreflightMode(s string) awarenesspb.PreflightMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "standard", "deep":
		return awarenesspb.PreflightMode_PREFLIGHT_STANDARD
	default:
		return awarenesspb.PreflightMode_PREFLIGHT_COMPACT
	}
}

func emitPreflightJSON(r *awarenesspb.PreflightResponse) error {
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preflight response: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

func printPreflightHuman(r *awarenesspb.PreflightResponse) {
	fmt.Printf("Status:      %s\n", trimPreflightStatusPrefix(r.GetStatus().String()))
	fmt.Printf("Risk class:  %s\n", trimRiskClassPrefix(r.GetRiskClass().String()))
	fmt.Printf("Confidence:  %s\n", trimConfidencePrefix(r.GetConfidence().String()))

	cov := r.GetCoverage()
	fmt.Printf("Coverage:    sufficient=%t  direct=%d  files=%d  indexed=%d\n",
		cov.GetSufficient(),
		cov.GetDirectAnchorCount(),
		cov.GetFileCount(),
		cov.GetIndexedFileCount(),
	)
	if note := cov.GetNote(); note != "" {
		fmt.Printf("             %s\n", note)
	}

	printPreflightList("Required actions", r.GetRequiredActions())
	printPreflightList("Files to read", r.GetFilesToRead())
	printPreflightList("Tests to run", r.GetTestsToRun())
	printPreflightList("Forbidden fixes", r.GetForbiddenFixes())
	printPreflightList("Blind spots", r.GetBlindSpots())

	if pats := r.GetImplementationPatterns(); len(pats) > 0 {
		fmt.Println("\nImplementation patterns:")
		for _, p := range pats {
			fmt.Printf("  - %s [%s]\n", trimAwarenessIDPrefix(p.GetId()), p.GetMatchStrength())
			for _, reason := range p.GetMatchReason() {
				fmt.Printf("      %s\n", reason)
			}
		}
	}

	fmt.Printf("\nGenerated in: %d ms\n", r.GetGeneratedInMs())
}

func printPreflightList(label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", label)
	for _, s := range items {
		fmt.Printf("  - %s\n", s)
	}
}

// trim*Prefix helpers turn the verbose proto enum names into short
// human-friendly tokens (e.g. PREFLIGHT_STATUS_OK → ok).
func trimPreflightStatusPrefix(s string) string {
	return strings.ToLower(strings.TrimPrefix(s, "PREFLIGHT_STATUS_"))
}

func trimRiskClassPrefix(s string) string {
	return strings.ToLower(s)
}

func trimConfidencePrefix(s string) string {
	return strings.ToLower(strings.TrimPrefix(s, "CONFIDENCE_"))
}

func trimAwarenessIDPrefix(s string) string {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func init() {
	awarenessPreflightCmd.Flags().StringVarP(&preflightTask, "task", "t", "",
		"Free-form task description (at least one of --task or --file is required)")
	awarenessPreflightCmd.Flags().StringSliceVarP(&preflightFiles, "file", "f", nil,
		"Repo-relative path (repeat the flag for multiple files)")
	awarenessPreflightCmd.Flags().StringVarP(&preflightMode, "mode", "m", "compact",
		"compact (top-3 entries) | standard (top-7 entries)")
	awarenessPreflightCmd.Flags().BoolVar(&preflightJSON, "json", false,
		"Emit the response as JSON instead of human-readable text")

	awarenessCmd.AddCommand(awarenessPreflightCmd)
}

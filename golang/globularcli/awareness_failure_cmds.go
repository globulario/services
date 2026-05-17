package main

// awareness_failure_cmds.go: globular awareness failure <subcommand>
//
// Commands:
//
//	globular awareness failure match        --error "<raw>" [--component <c>] [--service <s>] [--file <f>] [--json]
//	globular awareness failure explain      --category <id-or-name>              [--json]
//	globular awareness failure learn-incident --incident <id> --category <name> ...
//	globular awareness failure list-categories                                    [--json]
//	globular awareness failure seed-defaults
//	globular awareness failure similar      --error "<raw>" [--component <c>]    [--json]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/globulario/services/golang/awareness/failuregraph"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/spf13/cobra"
)

var failureCmd = &cobra.Command{
	Use:   "failure",
	Short: "Failure Knowledge Graph: match, explain, learn, list, seed",
}

func init() {
	awarenessCmd.AddCommand(failureCmd)
	failureCmd.AddCommand(
		failureMatchCmd,
		failureExplainCmd,
		failureLearnIncidentCmd,
		failureListCategoriesCmd,
		failureSeedDefaultsCmd,
		failureSimilarCmd,
	)
}

var failureCfg = struct {
	rawError    string
	component   string
	service     string
	filePath    string
	categoryID  string
	incidentID  string
	categoryName string
	symptoms    []string
	causes      []string
	resolutions []string
	wrongFixes  []string
	tests       []string
	limit       int
	jsonOutput  bool
}{}

func openFailureGraph() (*graph.Graph, *failuregraph.Store, error) {
	const systemPath = "/var/lib/globular/awareness/graph.json"
	if _, err := os.Stat(systemPath); err != nil {
		return nil, nil, fmt.Errorf("awareness graph not found — run 'globular awareness build' first")
	}
	g, err := graph.Open(systemPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open awareness graph: %w", err)
	}
	return g, failuregraph.New(g), nil
}

// ── match ─────────────────────────────────────────────────────────────────────

var failureMatchCmd = &cobra.Command{
	Use:   "match",
	Short: "Match a raw error string against the Failure Knowledge Graph",
	Example: `  globular awareness failure match \
    --error "x509: certificate is valid for globule-ryzen.globular.internal, not 10.0.0.100" \
    --component cluster-controller`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if failureCfg.rawError == "" {
			return fmt.Errorf("--error is required")
		}
		g, s, err := openFailureGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		exp, err := failuregraph.MatchError(context.Background(), s, failuregraph.MatchErrorRequest{
			RawError:    failureCfg.rawError,
			Component:   failureCfg.component,
			ServiceName: failureCfg.service,
			FilePath:    failureCfg.filePath,
		})
		if err != nil {
			return err
		}
		if exp == nil {
			fmt.Fprintln(os.Stdout, "No confident match found in Failure Knowledge Graph.")
			return nil
		}
		if failureCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(exp)
		}
		printExplanation(os.Stdout, *exp)
		return nil
	},
}

// ── explain ───────────────────────────────────────────────────────────────────

var failureExplainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Explain a failure category by ID or name",
	Example: `  globular awareness failure explain --category installed_state_build_id_missing`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if failureCfg.categoryID == "" {
			return fmt.Errorf("--category is required")
		}
		g, s, err := openFailureGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := context.Background()
		catID := failureCfg.categoryID
		if !strings.HasPrefix(catID, "ERRCAT-") {
			catID = "ERRCAT-" + catID
		}
		exp, err := failuregraph.ExplainCategory(ctx, s, catID)
		if err != nil {
			return fmt.Errorf("category not found: %w", err)
		}
		if failureCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(exp)
		}
		printExplanation(os.Stdout, *exp)
		return nil
	},
}

// ── learn-incident ────────────────────────────────────────────────────────────

var failureLearnIncidentCmd = &cobra.Command{
	Use:   "learn-incident",
	Short: "Extract failure knowledge from an incident and store it in the graph",
	Example: `  globular awareness failure learn-incident \
    --incident INC-2026-0007 \
    --category vip_used_as_member_endpoint \
    --cause "PrimaryIP returned VIP" \
    --resolution "Use StableIP(clusterVIP)"`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if failureCfg.incidentID == "" || failureCfg.categoryName == "" {
			return fmt.Errorf("--incident and --category are required")
		}
		g, s, err := openFailureGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		nodes, edges, err := failuregraph.LearnFromIncident(
			context.Background(), s,
			failureCfg.incidentID, failureCfg.categoryName,
			failureCfg.symptoms, failureCfg.causes,
			failureCfg.resolutions, failureCfg.wrongFixes,
			failureCfg.tests,
		)
		if err != nil {
			return err
		}
		if failureCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"status":         "learned",
				"created_nodes":  nodes,
				"created_edges":  edges,
				"categories":     []string{failureCfg.categoryName},
			})
		}
		fmt.Fprintf(os.Stdout, "Learned from %s: %d nodes, %d edges created/updated.\n",
			failureCfg.incidentID, nodes, edges)
		return nil
	},
}

// ── list-categories ───────────────────────────────────────────────────────────

var failureListCategoriesCmd = &cobra.Command{
	Use:   "list-categories",
	Short: "List all known failure categories",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, s, err := openFailureGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		cats, err := s.ListCategories(context.Background())
		if err != nil {
			return err
		}
		if failureCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(cats)
		}
		if len(cats) == 0 {
			fmt.Fprintln(os.Stdout, "No failure categories found. Run 'globular awareness failure seed-defaults' first.")
			return nil
		}
		fmt.Fprintf(os.Stdout, "FAILURE CATEGORIES (%d)\n\n", len(cats))
		for _, cat := range cats {
			fmt.Fprintf(os.Stdout, "  %-50s  [%s]\n", cat.Name, cat.Severity)
			if cat.Summary != "" {
				fmt.Fprintf(os.Stdout, "  %s\n\n", wrapAt(cat.Summary, 70))
			}
		}
		return nil
	},
}

// ── seed-defaults ─────────────────────────────────────────────────────────────

var failureSeedDefaultsCmd = &cobra.Command{
	Use:   "seed-defaults",
	Short: "Seed the Failure Knowledge Graph with built-in failure categories",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, s, err := openFailureGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		n, err := failuregraph.SeedDefaults(context.Background(), s)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Seeded %d failure categories.\n", n)
		return nil
	},
}

// ── similar ───────────────────────────────────────────────────────────────────

var failureSimilarCmd = &cobra.Command{
	Use:   "similar",
	Short: "Find failure categories similar to a given error",
	Example: `  globular awareness failure similar --error "unexpected end of JSON input"`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if failureCfg.rawError == "" {
			return fmt.Errorf("--error is required")
		}
		g, s, err := openFailureGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		results, err := failuregraph.FindSimilar(context.Background(), s, failuregraph.SimilarFailureRequest{
			RawError:  failureCfg.rawError,
			Component: failureCfg.component,
			Limit:     failureCfg.limit,
		})
		if err != nil {
			return err
		}
		if failureCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"matches": results})
		}
		if len(results) == 0 {
			fmt.Fprintln(os.Stdout, "No similar failures found.")
			return nil
		}
		for i, exp := range results {
			fmt.Fprintf(os.Stdout, "── Match %d: %s (%s, score %.2f) ──\n",
				i+1, exp.Category.Name, exp.Confidence, exp.Score)
			if exp.RecommendedAction != "" {
				fmt.Fprintf(os.Stdout, "  %s\n\n", exp.RecommendedAction)
			}
		}
		return nil
	},
}

// ── flag registration ─────────────────────────────────────────────────────────

func init() {
	failureMatchCmd.Flags().StringVar(&failureCfg.rawError, "error", "", "Raw error string to match")
	failureMatchCmd.Flags().StringVar(&failureCfg.component, "component", "", "Component or package name")
	failureMatchCmd.Flags().StringVar(&failureCfg.service, "service", "", "Service name")
	failureMatchCmd.Flags().StringVar(&failureCfg.filePath, "file", "", "Source file path")
	failureMatchCmd.Flags().BoolVar(&failureCfg.jsonOutput, "json", false, "JSON output")

	failureExplainCmd.Flags().StringVar(&failureCfg.categoryID, "category", "", "Category ID or name")
	failureExplainCmd.Flags().BoolVar(&failureCfg.jsonOutput, "json", false, "JSON output")

	failureLearnIncidentCmd.Flags().StringVar(&failureCfg.incidentID, "incident", "", "Incident ID")
	failureLearnIncidentCmd.Flags().StringVar(&failureCfg.categoryName, "category", "", "Failure category name")
	failureLearnIncidentCmd.Flags().StringArrayVar(&failureCfg.symptoms, "symptom", nil, "Observed symptom (repeatable)")
	failureLearnIncidentCmd.Flags().StringArrayVar(&failureCfg.causes, "cause", nil, "Root cause (repeatable)")
	failureLearnIncidentCmd.Flags().StringArrayVar(&failureCfg.resolutions, "resolution", nil, "Resolution applied (repeatable)")
	failureLearnIncidentCmd.Flags().StringArrayVar(&failureCfg.wrongFixes, "wrong-fix", nil, "Wrong fix to avoid (repeatable)")
	failureLearnIncidentCmd.Flags().StringArrayVar(&failureCfg.tests, "test", nil, "Regression test (repeatable)")
	failureLearnIncidentCmd.Flags().BoolVar(&failureCfg.jsonOutput, "json", false, "JSON output")

	failureListCategoriesCmd.Flags().BoolVar(&failureCfg.jsonOutput, "json", false, "JSON output")

	failureSimilarCmd.Flags().StringVar(&failureCfg.rawError, "error", "", "Raw error string")
	failureSimilarCmd.Flags().StringVar(&failureCfg.component, "component", "", "Component name")
	failureSimilarCmd.Flags().IntVar(&failureCfg.limit, "limit", 5, "Maximum results")
	failureSimilarCmd.Flags().BoolVar(&failureCfg.jsonOutput, "json", false, "JSON output")
}

// ── output helpers ────────────────────────────────────────────────────────────

func printExplanation(w *os.File, exp failuregraph.FailureExplanation) {
	fmt.Fprintf(w, "FAILURE MATCH\n\n")
	fmt.Fprintf(w, "Category:\n  %s\n\n", exp.Category.Name)
	if exp.Confidence != "" {
		fmt.Fprintf(w, "Confidence: %s (score %.2f)\n\n", exp.Confidence, exp.Score)
	}
	if exp.Category.Summary != "" {
		fmt.Fprintf(w, "Summary:\n  %s\n\n", exp.Category.Summary)
	}
	if len(exp.LikelyCauses) > 0 {
		fmt.Fprintln(w, "Likely cause:")
		for _, c := range exp.LikelyCauses {
			fmt.Fprintf(w, "  - %s\n", c.Summary)
		}
		fmt.Fprintln(w)
	}
	if len(exp.Resolutions) > 0 {
		fmt.Fprintln(w, "Known resolution:")
		for _, r := range exp.Resolutions {
			fmt.Fprintf(w, "  - %s\n", r.Summary)
		}
		fmt.Fprintln(w)
	}
	if len(exp.WrongFixes) > 0 {
		fmt.Fprintln(w, "Wrong fixes to avoid:")
		for _, wf := range exp.WrongFixes {
			fmt.Fprintf(w, "  - %s\n", wf.Summary)
		}
		fmt.Fprintln(w)
	}
	if len(exp.RequiredTests) > 0 {
		fmt.Fprintln(w, "Required regression tests:")
		for _, t := range exp.RequiredTests {
			fmt.Fprintf(w, "  - %s\n", t.Summary)
		}
		fmt.Fprintln(w)
	}
	if exp.RecommendedAction != "" {
		fmt.Fprintf(w, "Recommended action:\n  %s\n", exp.RecommendedAction)
	}
}

func wrapAt(s string, width int) string {
	if len(s) <= width {
		return s
	}
	return s[:width] + "..."
}

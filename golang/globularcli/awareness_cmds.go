package main

// awareness_cmds.go — CLI front-end for the awareness-graph gRPC service.
//
// All four subcommands forward to github.com/globulario/awareness-graph via
// the shared thin client in golang/awareness_graph_client. Output is shaped
// for human reading (tab-aligned tables, prose, indented anchors); machine
// consumers should call the gRPC service directly or use the MCP awareness.*
// tools.
//
// Usage:
//
//	globular awareness briefing [--file <path> | --task "<text>"] [--depth compact|standard|deep]
//	globular awareness impact <file>
//	globular awareness resolve <class> <id>
//	globular awareness query --mode by_file|by_id|by_class|related [flags...]
//
// Examples:
//
//	globular awareness briefing --file golang/cluster_controller/cluster_controller_server/server.go
//	globular awareness impact golang/repository/repository_server/sync_from_upstream.go
//	globular awareness resolve Invariant reconcile.dep_block_records_must_be_cleared_when_dep_satisfies
//	globular awareness query --mode by_class --class incident_pattern --limit 20

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	"github.com/globulario/services/golang/awareness_graph_client"
)

// awarenessAddrOverride lets an operator point the CLI at a specific
// awareness-graph instance (useful during local dev or when probing a
// specific node). When empty, the address is resolved from etcd.
var awarenessAddrOverride string

// awarenessInsecure disables TLS for the gRPC dial. Localhost-dev only;
// the standalone awareness-graph defaults to plaintext, and this flag
// lets the CLI talk to it without TLS plumbing.
var awarenessInsecure bool

var awarenessCmd = &cobra.Command{
	Use:   "awareness",
	Short: "Query the awareness-graph service (briefing/impact/resolve/query)",
	Long: `Talk to the awareness-graph gRPC service.

Awareness is the project's compact map of intent, invariants, failure modes,
incident patterns, required tests, and forbidden fixes. Use it before editing
significant code; it does not replace reading code or running tests.

The four subcommands mirror the gRPC contract:
  briefing — prose summary for a file or task (start here)
  impact   — direct + inferred anchors that touch a file
  resolve  — full record of one node by class + id
  query    — typed browse (by_file|by_id|by_class|related)`,
}

// ─── briefing ───────────────────────────────────────────────────────────

var (
	briefingFile  string
	briefingTask  string
	briefingDepth string
)

var awarenessBriefingCmd = &cobra.Command{
	Use:   "briefing",
	Short: "Compose a prose briefing before editing a file or starting a task",
	Long: `Returns a prose summary of relevant rules, invariants, failure modes,
required tests, and forbidden fixes. Exactly one of --file or --task is required.

Depth controls the token budget:
  compact  (default) ~500 tokens
  standard           ~1500 tokens
  deep               ~4000 tokens

The output begins with a status line (ok|empty|degraded). Treat EMPTY as
"no direct anchors were found" — not as proof of safety.`,
	RunE: runAwarenessBriefing,
}

func runAwarenessBriefing(cmd *cobra.Command, args []string) error {
	if briefingFile == "" && briefingTask == "" {
		return fmt.Errorf("either --file or --task is required")
	}
	if briefingFile != "" && briefingTask != "" {
		return fmt.Errorf("--file and --task are mutually exclusive")
	}
	cli, err := awarenessDialClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := cli.Briefing(ctx, briefingFile, briefingTask, briefingDepth)
	if err != nil {
		return fmt.Errorf("briefing rpc: %w", err)
	}
	printBriefing(resp)
	return nil
}

func printBriefing(r *awarenesspb.BriefingResponse) {
	status := awarenessBriefingStatusStr(r.GetStatus())
	fmt.Printf("Status: %s   Generated in: %d ms\n", status, r.GetGeneratedInMs())
	if prose := strings.TrimSpace(r.GetProse()); prose != "" {
		fmt.Println()
		fmt.Println(prose)
	}
	if refs := r.GetReferencedIds(); len(refs) > 0 {
		fmt.Println()
		fmt.Println("Referenced IDs:")
		for _, id := range refs {
			fmt.Printf("  - %s\n", id)
		}
	}
}

// ─── impact ─────────────────────────────────────────────────────────────

var awarenessImpactCmd = &cobra.Command{
	Use:   "impact <file>",
	Short: "List direct + inferred awareness anchors for a file",
	Args:  cobra.ExactArgs(1),
	RunE:  runAwarenessImpact,
}

func runAwarenessImpact(cmd *cobra.Command, args []string) error {
	cli, err := awarenessDialClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := cli.Impact(ctx, args[0])
	if err != nil {
		return fmt.Errorf("impact rpc: %w", err)
	}
	printImpact(args[0], resp)
	return nil
}

func printImpact(file string, r *awarenesspb.ImpactResponse) {
	fmt.Printf("Impact for: %s\n", file)
	sections := []struct {
		title string
		nodes []*awarenesspb.KnowledgeNode
	}{
		{"Direct invariants", r.GetDirectInvariants()},
		{"Direct failure modes", r.GetDirectFailureModes()},
		{"Direct incident patterns", r.GetDirectIncidentPatterns()},
		{"Direct intents", r.GetDirectIntents()},
		{"Inferred invariants", r.GetInferredInvariants()},
		{"Inferred failure modes", r.GetInferredFailureModes()},
		{"Inferred incident patterns", r.GetInferredIncidentPatterns()},
		{"Inferred intents", r.GetInferredIntents()},
		{"Required tests", r.GetRequiredTests()},
		{"Forbidden fixes", r.GetForbiddenFixes()},
	}
	any := false
	for _, sec := range sections {
		if len(sec.nodes) == 0 {
			continue
		}
		any = true
		fmt.Println()
		fmt.Printf("%s (%d):\n", sec.title, len(sec.nodes))
		for _, n := range sec.nodes {
			line := fmt.Sprintf("  - %s:%s", n.GetClass(), n.GetId())
			if label := n.GetLabel(); label != "" {
				line += "  — " + label
			}
			if sev := n.GetSeverity(); sev != "" {
				line += fmt.Sprintf("  [%s]", sev)
			}
			fmt.Println(line)
		}
	}
	if !any {
		fmt.Println("(no direct or inferred anchors found — author awareness or treat as low-coverage)")
	}
}

// ─── resolve ────────────────────────────────────────────────────────────

var awarenessResolveCmd = &cobra.Command{
	Use:   "resolve <class> <id>",
	Short: "Fetch a single awareness node by class + bare id",
	Long: `Class is one of: Invariant, FailureMode, IncidentPattern, Intent,
ForbiddenFix, Test, SourceFile, Symbol, EtcdKey, SystemdUnit.

Examples:
  globular awareness resolve Invariant reconcile.dep_block_records_must_be_cleared_when_dep_satisfies
  globular awareness resolve FailureMode service.runtime_identity_unproven`,
	Args: cobra.ExactArgs(2),
	RunE: runAwarenessResolve,
}

func runAwarenessResolve(cmd *cobra.Command, args []string) error {
	cli, err := awarenessDialClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.Resolve(ctx, args[0], args[1])
	if err != nil {
		return fmt.Errorf("resolve rpc: %w", err)
	}
	if !resp.GetFound() {
		fmt.Printf("not found: %s:%s\n", args[0], args[1])
		os.Exit(2)
	}
	printNode(resp.GetNode())
	return nil
}

func printNode(n *awarenesspb.KnowledgeNode) {
	if n == nil {
		fmt.Println("(empty node)")
		return
	}
	fmt.Printf("%s:%s\n", n.GetClass(), n.GetId())
	if v := n.GetLabel(); v != "" {
		fmt.Printf("  Label:       %s\n", v)
	}
	if v := n.GetSeverity(); v != "" {
		fmt.Printf("  Severity:    %s\n", v)
	}
	if v := n.GetStatus(); v != "" {
		fmt.Printf("  Status:      %s\n", v)
	}
	if v := n.GetDescription(); v != "" {
		fmt.Printf("  Description: %s\n", strings.TrimSpace(v))
	}
	if v := n.GetIri(); v != "" {
		fmt.Printf("  IRI:         %s\n", v)
	}
	if a := n.GetAnchor(); a != nil {
		if src := a.GetSourceYaml(); src != "" {
			fmt.Printf("  Source YAML: %s\n", src)
		}
		if f := a.GetFile(); f != "" {
			loc := f
			if a.GetLineStart() != 0 {
				loc = fmt.Sprintf("%s:%d", f, a.GetLineStart())
				if a.GetLineEnd() != 0 && a.GetLineEnd() != a.GetLineStart() {
					loc += fmt.Sprintf("-%d", a.GetLineEnd())
				}
			}
			fmt.Printf("  Code anchor: %s\n", loc)
		}
		if sym := a.GetSymbol(); sym != "" {
			fmt.Printf("  Symbol:      %s\n", sym)
		}
	}
	if rel := n.GetRelatedIds(); len(rel) > 0 {
		fmt.Println("  Related:")
		for _, id := range rel {
			fmt.Printf("    - %s\n", id)
		}
	}
}

// ─── query ──────────────────────────────────────────────────────────────

var (
	queryMode  string
	queryFile  string
	queryID    string
	queryClass string
	queryLimit int
)

var awarenessQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Structured browse: by_file | by_id | by_class | related",
	Long: `Modes:
  by_file   list nodes whose anchor names --file
  by_id     return the node matching --id (class-qualified, e.g. invariant:foo)
  by_class  list all nodes of --class (use --limit; default 50)
  related   list nodes pointed at by --id

Use sparingly. For a single rule, prefer 'resolve'.
For a file's full anchor surface, prefer 'impact'.`,
	RunE: runAwarenessQuery,
}

func runAwarenessQuery(cmd *cobra.Command, args []string) error {
	mode, ok := cliQueryMode(queryMode)
	if !ok {
		return fmt.Errorf("--mode must be one of: by_file, by_id, by_class, related")
	}
	req := &awarenesspb.QueryRequest{
		Mode:  mode,
		File:  queryFile,
		Id:    queryID,
		Limit: int32(queryLimit),
	}
	if queryClass != "" {
		cls, ok := cliQueryClass(queryClass)
		if !ok {
			return fmt.Errorf("--class must be one of: invariant, failure_mode, incident_pattern, intent, symbol, source_file")
		}
		req.Class = cls
	}

	cli, err := awarenessDialClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.Query(ctx, req)
	if err != nil {
		return fmt.Errorf("query rpc: %w", err)
	}
	printQueryRows(resp.GetRows())
	return nil
}

func printQueryRows(rows []*awarenesspb.QueryRow) {
	if len(rows) == 0 {
		fmt.Println("(no rows)")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CLASS\tID\tLABEL\tSEVERITY\tSTATUS\tRELATION\tSOURCE")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.GetClass(), r.GetId(),
			awarenessTruncate(r.GetLabel(), 60),
			r.GetSeverity(), r.GetStatus(),
			r.GetRelation(), r.GetSourceFile(),
		)
	}
	_ = w.Flush()
	fmt.Printf("\n%d row(s)\n", len(rows))
}

func awarenessTruncate(s string, n int) string { //nolint:unused // used in query row printer above
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ─── shared helpers ─────────────────────────────────────────────────────

func awarenessDialClient() (*awareness_graph_client.Client, error) {
	var opts []awareness_graph_client.Option
	if awarenessInsecure {
		opts = append(opts, awareness_graph_client.WithInsecure())
	}
	cli, err := awareness_graph_client.New(awarenessAddrOverride, opts...)
	if err != nil {
		return nil, fmt.Errorf("awareness-graph unreachable: %w (set --awareness-addr or deploy the service)", err)
	}
	return cli, nil
}

func awarenessBriefingStatusStr(s awarenesspb.BriefingStatus) string {
	switch s {
	case awarenesspb.BriefingStatus_BRIEFING_STATUS_OK:
		return "ok"
	case awarenesspb.BriefingStatus_BRIEFING_STATUS_EMPTY:
		return "empty"
	case awarenesspb.BriefingStatus_BRIEFING_STATUS_DEGRADED:
		return "degraded"
	default:
		return "unknown"
	}
}

func cliQueryMode(s string) (awarenesspb.QueryMode, bool) {
	switch s {
	case "by_file":
		return awarenesspb.QueryMode_QUERY_MODE_BY_FILE, true
	case "by_id":
		return awarenesspb.QueryMode_QUERY_MODE_BY_ID, true
	case "by_class":
		return awarenesspb.QueryMode_QUERY_MODE_BY_CLASS, true
	case "related":
		return awarenesspb.QueryMode_QUERY_MODE_RELATED, true
	}
	return 0, false
}

func cliQueryClass(s string) (awarenesspb.QueryClass, bool) {
	switch s {
	case "invariant":
		return awarenesspb.QueryClass_QUERY_CLASS_INVARIANT, true
	case "failure_mode":
		return awarenesspb.QueryClass_QUERY_CLASS_FAILURE_MODE, true
	case "incident_pattern":
		return awarenesspb.QueryClass_QUERY_CLASS_INCIDENT_PATTERN, true
	case "intent":
		return awarenesspb.QueryClass_QUERY_CLASS_INTENT, true
	case "symbol":
		return awarenesspb.QueryClass_QUERY_CLASS_SYMBOL, true
	case "source_file":
		return awarenesspb.QueryClass_QUERY_CLASS_SOURCE_FILE, true
	}
	return 0, false
}

// ─── wiring ─────────────────────────────────────────────────────────────

func init() {
	awarenessBriefingCmd.Flags().StringVarP(&briefingFile, "file", "f", "", "Repo-relative path (mutually exclusive with --task)")
	awarenessBriefingCmd.Flags().StringVarP(&briefingTask, "task", "t", "", "Free-form task description (mutually exclusive with --file)")
	awarenessBriefingCmd.Flags().StringVarP(&briefingDepth, "depth", "d", "compact", "compact | standard | deep")

	awarenessQueryCmd.Flags().StringVar(&queryMode, "mode", "", "by_file | by_id | by_class | related (required)")
	awarenessQueryCmd.Flags().StringVar(&queryFile, "file", "", "Repo-relative path (required for mode=by_file)")
	awarenessQueryCmd.Flags().StringVar(&queryID, "id", "", "Class-qualified id (required for mode=by_id / mode=related)")
	awarenessQueryCmd.Flags().StringVar(&queryClass, "class", "", "Class (required for mode=by_class)")
	awarenessQueryCmd.Flags().IntVar(&queryLimit, "limit", 50, "Maximum rows")
	_ = awarenessQueryCmd.MarkFlagRequired("mode")

	awarenessCmd.PersistentFlags().StringVar(&awarenessAddrOverride, "awareness-addr", "",
		"Override awareness-graph address (host:port). Defaults to etcd discovery.")
	awarenessCmd.PersistentFlags().BoolVar(&awarenessInsecure, "awareness-insecure", false,
		"Disable TLS for the awareness-graph dial. Localhost-dev only.")

	awarenessCmd.AddCommand(awarenessBriefingCmd)
	awarenessCmd.AddCommand(awarenessImpactCmd)
	awarenessCmd.AddCommand(awarenessResolveCmd)
	awarenessCmd.AddCommand(awarenessQueryCmd)

	rootCmd.AddCommand(awarenessCmd)
}

package main

// awareness_close_incident_cmd.go: globular awareness close-incident
//
// One command that performs the post-fix closure ritual in a single graph
// open. Today the ritual requires two separate CLI invocations
// (`failure learn-incident` + `incident-pattern record`) plus a hand-edited
// JSON pattern file. The two-step shape and the permission gotcha on
// /var/lib/globular/awareness/graph.json are why the failure graph stays
// sparse — the friction wins and the closure step gets skipped.
//
// This command reads ONE spec (YAML or JSON), validates it up front,
// probes write access, then runs both writes atomically against a single
// graph open. The permission failure mode is reported as an actionable
// hint instead of a raw "permission denied".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/globulario/services/golang/awareness/failuregraph"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/incidentpattern"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// closeIncidentSpec is the merged YAML/JSON shape accepted by close-incident.
// It carries both the failure-knowledge fields (consumed by
// failuregraph.LearnFromIncident) and the optional incident-pattern fields
// (consumed by incidentpattern.Store.RecordPattern). YAML tags use snake_case
// to match what's already shown in `incident-pattern record` examples.
type closeIncidentSpec struct {
	IncidentID  string   `yaml:"incident_id"  json:"incident_id"`
	Category    string   `yaml:"category"     json:"category"`
	Symptoms    []string `yaml:"symptoms"     json:"symptoms,omitempty"`
	Causes      []string `yaml:"causes"       json:"causes,omitempty"`
	Resolutions []string `yaml:"resolutions"  json:"resolutions,omitempty"`
	WrongFixes  []string `yaml:"wrong_fixes"  json:"wrong_fixes,omitempty"`
	Tests       []string `yaml:"tests"        json:"tests,omitempty"`

	Pattern *closeIncidentPatternSpec `yaml:"pattern,omitempty" json:"pattern,omitempty"`
}

type closeIncidentPatternSpec struct {
	Title       string                          `yaml:"title"        json:"title"`
	Severity    string                          `yaml:"severity"     json:"severity,omitempty"`
	Summary     string                          `yaml:"summary"      json:"summary,omitempty"`
	FailureMode string                          `yaml:"failure_mode" json:"failure_mode,omitempty"`
	RootCause   string                          `yaml:"root_cause"   json:"root_cause,omitempty"`
	Lesson      string                          `yaml:"lesson"       json:"lesson,omitempty"`
	Files       []incidentpattern.PatternFile   `yaml:"files,omitempty"        json:"files,omitempty"`
	Symbols     []incidentpattern.PatternSymbol `yaml:"symbols,omitempty"      json:"symbols,omitempty"`
	Invariants  []incidentpattern.PatternInvariant `yaml:"invariants,omitempty" json:"invariants,omitempty"`
	EditShapes  []incidentpattern.EditShape     `yaml:"edit_shapes,omitempty"  json:"edit_shapes,omitempty"`
	FailedFixes []incidentpattern.FailedFix     `yaml:"failed_fixes,omitempty" json:"failed_fixes,omitempty"`
}

var closeIncidentCfg = struct {
	specPath   string
	useStdin   bool
	dbPath     string
	dryRun     bool
	jsonOutput bool
}{}

var awarenessCloseIncidentCmd = &cobra.Command{
	Use:   "close-incident",
	Short: "Close the loop on a fixed incident in a single command (failure-graph + incident-pattern)",
	Long: `Run the post-fix closure ritual atomically.

The closure ritual is mandatory after every fix (see CLAUDE.md §7) but today
it takes two CLI invocations and an extra JSON file. This command merges them:
one YAML/JSON spec, one graph open, both writes or neither.

Example spec (close.yaml):
  incident_id: INC-2026-0042
  category: split_authoritative_state_transition
  symptoms:
    - "Install retry loop after leader failover"
  causes:
    - "dispatchInstallResult split result+ack across two etcd writes"
  resolutions:
    - "Collapse result+ack into a single etcd transaction"
  wrong_fixes:
    - "Adding a retry loop around the second write (masks the race)"
  tests:
    - "TestInstallResultAtomic in cluster_controller/reconcile_test.go"
  pattern:
    title: "etcd cascade after partial install result promotion"
    severity: critical
    summary: "Result promotion split across multiple etcd writes."
    failure_mode: partial_authoritative_state_commit
    root_cause: "Two writes, no transaction, leader failover between them."
    lesson: "Result promotion and ack must commit atomically."
    files:
      - path: golang/cluster_controller/reconcile.go
        role: dispatch authority
    edit_shapes:
      - shape_kind: split_authoritative_state_transition
        description: "Authoritative state written in N>1 etcd ops"
        dangerous: true

Usage:
  globular awareness close-incident --spec close.yaml
  cat close.yaml | globular awareness close-incident --stdin
  globular awareness close-incident --spec close.yaml --dry-run   # validate only
  globular awareness close-incident --spec close.yaml --json      # machine-readable

The graph file at /var/lib/globular/awareness/graph.json is owned by root,
so the closure typically requires either sudo or membership in a writable
group. If the write probe fails, this command prints an actionable hint
instead of "permission denied".`,
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          runCloseIncident,
}

func init() {
	awarenessCmd.AddCommand(awarenessCloseIncidentCmd)
	awarenessCloseIncidentCmd.Flags().StringVar(&closeIncidentCfg.specPath, "spec", "", "Path to a YAML or JSON closure spec")
	awarenessCloseIncidentCmd.Flags().BoolVar(&closeIncidentCfg.useStdin, "stdin", false, "Read closure spec from stdin")
	awarenessCloseIncidentCmd.Flags().StringVar(&closeIncidentCfg.dbPath, "db", "", "Override graph.json path (default /var/lib/globular/awareness/graph.json)")
	awarenessCloseIncidentCmd.Flags().BoolVar(&closeIncidentCfg.dryRun, "dry-run", false, "Validate the spec and probe write access; do not modify the graph")
	awarenessCloseIncidentCmd.Flags().BoolVar(&closeIncidentCfg.jsonOutput, "json", false, "Emit a machine-readable JSON summary on success")
}

// defaultGraphPath mirrors the path hardcoded by openFailureGraph; we keep
// it here rather than exposing it from awareness_failure_cmds.go because
// close-incident is the only caller that needs an early write probe.
const defaultGraphPath = "/var/lib/globular/awareness/graph.json"

func runCloseIncident(cmd *cobra.Command, _ []string) error {
	spec, err := readCloseIncidentSpec()
	if err != nil {
		return err
	}
	if err := spec.validate(); err != nil {
		return err
	}

	dbPath := closeIncidentCfg.dbPath
	if dbPath == "" {
		dbPath = defaultGraphPath
	}

	if err := probeGraphWritable(dbPath); err != nil {
		return err
	}

	if closeIncidentCfg.dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "dry-run ok: spec valid, %s writable\n", dbPath)
		return nil
	}

	g, err := graph.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open graph %s: %w", dbPath, err)
	}
	defer g.Close()

	ctx := context.Background()

	nodes, edges, err := failuregraph.LearnFromIncident(
		ctx, failuregraph.New(g),
		spec.IncidentID, spec.Category,
		spec.Symptoms, spec.Causes, spec.Resolutions, spec.WrongFixes, spec.Tests,
	)
	if err != nil {
		return fmt.Errorf("learn-incident: %w", err)
	}

	var patternID string
	if spec.Pattern != nil {
		stored, err := incidentpattern.NewStore(g).RecordPattern(ctx, spec.Pattern.toModel(spec.IncidentID))
		if err != nil {
			return fmt.Errorf("incident-pattern record: %w", err)
		}
		patternID = stored.ID
	}

	if closeIncidentCfg.jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"status":        "closed",
			"incident":      spec.IncidentID,
			"category":      spec.Category,
			"created_nodes": nodes,
			"created_edges": edges,
			"pattern_id":    patternID,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"closed %s: failure-graph +%d nodes / +%d edges; pattern=%s\n",
		spec.IncidentID, nodes, edges, ifEmpty(patternID, "(none)"))
	return nil
}

func ifEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func readCloseIncidentSpec() (*closeIncidentSpec, error) {
	var raw []byte
	switch {
	case closeIncidentCfg.useStdin && closeIncidentCfg.specPath != "":
		return nil, errors.New("--spec and --stdin are mutually exclusive")
	case closeIncidentCfg.useStdin:
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		raw = b
	case closeIncidentCfg.specPath != "":
		b, err := os.ReadFile(closeIncidentCfg.specPath)
		if err != nil {
			return nil, fmt.Errorf("read spec %s: %w", closeIncidentCfg.specPath, err)
		}
		raw = b
	default:
		return nil, errors.New("--spec <file> or --stdin is required")
	}

	var spec closeIncidentSpec
	// yaml.v3 accepts well-formed JSON as a YAML 1.2 superset, so the same
	// decoder handles both formats. No need to sniff the suffix.
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	return &spec, nil
}

func (s *closeIncidentSpec) validate() error {
	var missing []string
	if strings.TrimSpace(s.IncidentID) == "" {
		missing = append(missing, "incident_id")
	}
	if strings.TrimSpace(s.Category) == "" {
		missing = append(missing, "category")
	}
	if len(missing) > 0 {
		return fmt.Errorf("spec missing required field(s): %s", strings.Join(missing, ", "))
	}
	if s.Pattern != nil && strings.TrimSpace(s.Pattern.Title) == "" {
		return errors.New("pattern.title is required when a pattern block is present")
	}
	return nil
}

func (p *closeIncidentPatternSpec) toModel(incidentID string) incidentpattern.IncidentPattern {
	return incidentpattern.IncidentPattern{
		IncidentID:  incidentID,
		Title:       p.Title,
		Summary:     p.Summary,
		Severity:    p.Severity,
		FailureMode: p.FailureMode,
		RootCause:   p.RootCause,
		Lesson:      p.Lesson,
		Files:       p.Files,
		Symbols:     p.Symbols,
		Invariants:  p.Invariants,
		EditShapes:  p.EditShapes,
		FailedFixes: p.FailedFixes,
	}
}

// probeGraphWritable opens the graph file for write without truncating or
// appending, just to surface EACCES early with a usable hint. Closes the
// descriptor immediately on success. The graph package only flushes on
// Close(), so without this probe a permission failure would only surface
// after both writes were applied to the in-memory graph — discarded silently.
func probeGraphWritable(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err == nil {
		_ = f.Close()
		return nil
	}
	if !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("graph file %s: %w", path, err)
	}
	return fmt.Errorf(`closure aborted: cannot write %s (permission denied).

This is the well-known closure-ritual gotcha — the graph file is owned by
root and a regular user cannot persist the failure/pattern updates.

Pick one:
  a) re-run under sudo:
       sudo globular awareness close-incident --spec <file>
  b) hand the directory to the globular group once and add yourself:
       sudo chgrp -R globular /var/lib/globular/awareness
       sudo chmod -R g+w     /var/lib/globular/awareness
       sudo usermod -aG globular $USER   # log out / back in for it to take`, path)
}

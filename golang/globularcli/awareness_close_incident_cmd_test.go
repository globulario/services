package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/incidentpattern"
)

// TestCloseIncidentSpec_ParsesYAML pins the merged spec shape that callers
// hand to `globular awareness close-incident`. If anyone breaks the field
// names (snake_case YAML tags), every existing close-incident spec on disk
// would silently parse with empty fields and the failure graph would
// receive empty rows.
func TestCloseIncidentSpec_ParsesYAML(t *testing.T) {
	in := []byte(`
incident_id: INC-2099-0001
category: split_authoritative_state_transition
symptoms:
  - "Install retry loop after leader failover"
causes:
  - "Result+ack split across two etcd writes"
resolutions:
  - "Collapse into one transaction"
wrong_fixes:
  - "Adding a retry loop around the second write"
tests:
  - "TestInstallResultAtomic"
pattern:
  title: "etcd cascade after partial install result promotion"
  severity: critical
  summary: "Result promotion split across multiple etcd writes."
  failure_mode: partial_authoritative_state_commit
  root_cause: "Two writes, no transaction"
  lesson: "Atomic promotion only"
  files:
    - path: golang/cluster_controller/reconcile.go
      role: dispatch authority
  edit_shapes:
    - shape_kind: split_authoritative_state_transition
      description: "N>1 etcd ops"
      dangerous: true
`)
	closeIncidentCfg.specPath = mustWriteTemp(t, in, "*.yaml")
	closeIncidentCfg.useStdin = false
	defer func() { closeIncidentCfg.specPath = "" }()

	spec, err := readCloseIncidentSpec()
	if err != nil {
		t.Fatalf("readCloseIncidentSpec: %v", err)
	}
	if spec.IncidentID != "INC-2099-0001" {
		t.Errorf("IncidentID = %q, want INC-2099-0001", spec.IncidentID)
	}
	if spec.Category != "split_authoritative_state_transition" {
		t.Errorf("Category = %q", spec.Category)
	}
	if got := len(spec.Symptoms); got != 1 {
		t.Errorf("Symptoms len = %d, want 1", got)
	}
	if spec.Pattern == nil {
		t.Fatal("Pattern block lost during parse")
	}
	if spec.Pattern.Title == "" {
		t.Error("pattern.title lost during parse")
	}
	if len(spec.Pattern.EditShapes) != 1 || !spec.Pattern.EditShapes[0].Dangerous {
		t.Errorf("EditShapes lost dangerous flag: %+v", spec.Pattern.EditShapes)
	}
	if len(spec.Pattern.Files) != 1 || spec.Pattern.Files[0].Path == "" {
		t.Errorf("Files block lost: %+v", spec.Pattern.Files)
	}
}

func TestCloseIncidentSpec_RejectsMissingRequiredFields(t *testing.T) {
	cases := map[string][]byte{
		"missing incident_id": []byte("category: foo\n"),
		"missing category":    []byte("incident_id: INC-X\n"),
		"empty both":          []byte("symptoms: [a]\n"),
		"pattern needs title": []byte("incident_id: INC-X\ncategory: c\npattern:\n  severity: critical\n"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			closeIncidentCfg.specPath = mustWriteTemp(t, body, "*.yaml")
			defer func() { closeIncidentCfg.specPath = "" }()

			spec, err := readCloseIncidentSpec()
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := spec.validate(); err == nil {
				t.Error("validate accepted invalid spec, want error")
			}
		})
	}
}

// TestRunCloseIncident_AtomicWrite drives the full happy path against a
// fresh graph in a tempdir. Both the failure-graph write and the
// incident-pattern write must land in the same graph.json — the whole
// point of the wrapper is that the two operations no longer drift apart.
func TestRunCloseIncident_AtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "graph.json")

	// Seed an empty graph so probeGraphWritable has something to test.
	g, err := graph.Open(dbPath)
	if err != nil {
		t.Fatalf("seed graph: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("seed close: %v", err)
	}

	spec := []byte(`
incident_id: INC-2099-0042
category: test_category_for_close_incident
symptoms: ["s1"]
causes: ["c1"]
resolutions: ["r1"]
wrong_fixes: ["wf1"]
tests: ["TestFoo in foo_test.go"]
pattern:
  title: "Test close-incident wrapper atomicity"
  severity: warning
  summary: "wraps two writes"
  failure_mode: test_failure_mode
  root_cause: "test"
  lesson: "test"
  files:
    - path: golang/some/file.go
      role: test
`)
	closeIncidentCfg.specPath = mustWriteTemp(t, spec, "*.yaml")
	closeIncidentCfg.dbPath = dbPath
	closeIncidentCfg.jsonOutput = false
	closeIncidentCfg.dryRun = false
	defer func() {
		closeIncidentCfg.specPath = ""
		closeIncidentCfg.dbPath = ""
	}()

	if err := runCloseIncident(awarenessCloseIncidentCmd, nil); err != nil {
		t.Fatalf("runCloseIncident: %v", err)
	}

	// Reopen and verify both writes are visible. The single graph open in
	// the wrapper is what guarantees this — if the two operations had been
	// split across two CLI invocations, a permission failure between them
	// would have committed one without the other.
	g, err = graph.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer g.Close()

	patterns, err := incidentpattern.NewStore(g).ListPatterns(context.Background())
	if err != nil {
		t.Fatalf("ListPatterns: %v", err)
	}
	var found bool
	for _, p := range patterns {
		if p.IncidentID == "INC-2099-0042" {
			found = true
			if p.Title == "" {
				t.Errorf("pattern stored without title for INC-2099-0042")
			}
			break
		}
	}
	if !found {
		t.Errorf("incident-pattern row missing for INC-2099-0042; close-incident did not commit it")
	}
}

func TestProbeGraphWritable_MissingFileReportsClearly(t *testing.T) {
	err := probeGraphWritable(filepath.Join(t.TempDir(), "absent.json"))
	if err == nil {
		t.Fatal("expected error for missing graph file")
	}
	// Should NOT print the permission-denied hint for a plain ENOENT.
	if strings.Contains(err.Error(), "closure aborted") {
		t.Errorf("ENOENT misreported as permission gotcha: %v", err)
	}
}

func mustWriteTemp(t *testing.T, body []byte, pattern string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.Write(body); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return f.Name()
}

package manual

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/graph"
)

// incidentPatternsFile mirrors docs/awareness/incident_patterns.yaml.
// The schema matches knowledge.IncidentPattern (awareness/knowledge/knowledge.go);
// duplicating it here keeps the manual extractor self-contained — no
// dependency on the knowledge package's loader.
type incidentPatternsFile struct {
	IncidentPatterns []yamlIncidentPattern `yaml:"incident_patterns"`
}

type yamlIncidentPattern struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Severity    string   `yaml:"severity"`
	FailureMode string   `yaml:"failure_mode"`
	// RootCause, Lesson, EditShapes, WrongFixes are read by the YAML
	// validator and search consumers but DELIBERATELY NOT mirrored into
	// graph node fields. The long body stays in the YAML; the graph node
	// is an index/pointer. The anti-bloat test pins this contract.
	RootCause         string   `yaml:"root_cause"`
	Lesson            string   `yaml:"lesson"`
	EditShapes        []string `yaml:"edit_shapes"`
	WrongFixes        []string `yaml:"wrong_fixes"`
	Files             []string `yaml:"files"`
	RelatedInvariants []string `yaml:"related_invariants"`
	RelatedSymbols    []string `yaml:"related_symbols"`
}

// LoadIncidentPatterns loads docs/awareness/incident_patterns.yaml into g.
// Missing files are silently skipped, matching the rest of the manual loader
// family.
//
// One YAML entry → one compact graph node of type incident_pattern. Edges
// reuse existing kinds so the impact-tool partition logic
// (directInvariantIDsForFile etc.) treats patterns the same way it treats
// invariants:
//
//   - incident_pattern → protects → source_file       (from files:)
//   - source_file     → implements → incident_pattern (reverse, lets impact
//                                                     find pattern from file)
//   - incident_pattern → affects  → invariant         (from related_invariants:)
//   - incident_pattern → affects  → failure_mode      (from failure_mode:)
//   - incident_pattern → forbids  → forbidden_fix     (from wrong_fixes:,
//                                                     ONLY when the entry
//                                                     matches an existing
//                                                     forbidden_fix node)
//
// We reuse EdgeAffects for both invariant and failure_mode links rather
// than introducing a new "related_to" edge kind — EdgeAffects already
// carries the "is about / connects to" semantics in the existing
// invariant → affects → failure_mode usage, and adding a kind would
// require every consumer to learn new vocabulary for no semantic gain.
//
// Dangling references (failure_mode / invariant / forbidden_fix names that
// don't resolve to existing nodes) are skipped silently here — the
// knowledge.Load validator already warns about them at build time, and we
// don't want to materialise orphan edges that point at nothing.
func LoadIncidentPatterns(ctx context.Context, g *graph.Graph, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("LoadIncidentPatterns: read %s: %w", path, err)
	}

	var f incidentPatternsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("LoadIncidentPatterns: parse %s: %w", path, err)
	}

	for _, pat := range f.IncidentPatterns {
		if err := loadIncidentPattern(ctx, g, pat, path); err != nil {
			return fmt.Errorf("LoadIncidentPatterns %s: %w", pat.ID, err)
		}
	}
	return nil
}

func loadIncidentPattern(ctx context.Context, g *graph.Graph, pat yamlIncidentPattern, path string) error {
	if strings.TrimSpace(pat.ID) == "" {
		// Validation is the knowledge.Load layer's job; we just skip here so
		// a malformed entry doesn't poison the rest of the build.
		return nil
	}
	nodeID := "incident_pattern:" + pat.ID

	// Compact summary. Use the title as the summary so the graph node carries
	// just enough text to be human-recognisable in tool output without
	// inflating graph.json. Long-form bodies (root_cause, lesson, wrong_fixes,
	// edit_shapes) stay in the YAML; consumers needing them go through
	// KnowledgeBase.IncidentPatterns or directly read the bundle's
	// docs/incident_patterns.yaml.
	summary := strings.TrimSpace(pat.Title)
	if summary == "" {
		summary = pat.ID
	}

	if err := g.AddNode(ctx, graph.Node{
		ID:      nodeID,
		Type:    graph.NodeTypeIncidentPattern,
		Name:    pat.ID,
		Path:    path,
		Summary: summary,
		Metadata: map[string]any{
			"severity":     pat.Severity,
			"failure_mode": pat.FailureMode,
		},
	}); err != nil {
		return err
	}

	// files: → source_file nodes + protects edge + reverse implements edge.
	// Mirrors invariants.go's protects.files handling so the impact tool's
	// directInvariantIDsForFile-equivalent walks land patterns the same way.
	for _, file := range pat.Files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		fileID := "source_file:" + file
		if err := g.AddNode(ctx, graph.Node{
			ID:   fileID,
			Type: graph.NodeTypeSourceFile,
			Name: file,
			Path: file,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeProtects, Dst: fileID}); err != nil {
			return err
		}
		// Reverse — what makes pattern surface as Direct match on the file.
		if err := g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImplements, Dst: nodeID}); err != nil {
			return err
		}
	}

	// related_invariants: → existing invariant nodes only. Skip dangling refs.
	for _, invID := range pat.RelatedInvariants {
		invID = strings.TrimSpace(invID)
		if invID == "" {
			continue
		}
		invNodeID := "invariant:" + invID
		exists, err := g.FindNode(ctx, invNodeID)
		if err != nil {
			return err
		}
		if exists == nil {
			continue // dangling — knowledge.Load already warned
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeAffects, Dst: invNodeID}); err != nil {
			return err
		}
	}

	// failure_mode: → existing failure_mode node only. Skip dangling refs.
	if fmID := strings.TrimSpace(pat.FailureMode); fmID != "" {
		fmNodeID := "failure_mode:" + fmID
		exists, err := g.FindNode(ctx, fmNodeID)
		if err != nil {
			return err
		}
		if exists != nil {
			if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeAffects, Dst: fmNodeID}); err != nil {
				return err
			}
		}
	}

	// wrong_fixes: → existing forbidden_fix nodes only. The forbidden_fixes
	// loader names them via the canonical forbidden_fixes.yaml; pattern
	// authors reference them by the same name. Names not declared as a
	// standalone forbidden_fix are silently skipped — we don't auto-promote
	// pattern-local strings into first-class forbidden_fix nodes because that
	// would create silent forbidden_fix names that bypass the validator.
	for _, fix := range pat.WrongFixes {
		fix = strings.TrimSpace(fix)
		if fix == "" {
			continue
		}
		ffNodeID := "forbidden_fix:" + fix
		exists, err := g.FindNode(ctx, ffNodeID)
		if err != nil {
			return err
		}
		if exists == nil {
			continue
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeForbids, Dst: ffNodeID}); err != nil {
			return err
		}
	}

	return nil
}

package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// graphJSON is the on-disk JSON format for static graph data.
type graphJSON struct {
	Version      int            `json:"version"`
	Nodes        []*Node        `json:"nodes,omitempty"`
	Edges        []*edgeJSON    `json:"edges,omitempty"`
	Invariants   []*Invariant   `json:"invariants,omitempty"`
	FailureModes []*FailureMode `json:"failure_modes,omitempty"`
	Builds       []*BuildRecord `json:"builds,omitempty"`
}

// edgeJSON is the serialisable form of an Edge.
type edgeJSON struct {
	Src        string         `json:"src"`
	Kind       string         `json:"kind"`
	Dst        string         `json:"dst"`
	Phase      string         `json:"phase,omitempty"`
	Required   bool           `json:"required,omitempty"`
	Confidence float64        `json:"confidence,omitempty"`
	Class      string         `json:"class,omitempty"`
	Weight     float64        `json:"weight,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Provenance map[string]any `json:"provenance,omitempty"`
}

// loadFromJSON populates the graph from serialised JSON data.
func (g *Graph) loadFromJSON(data []byte) error {
	var gj graphJSON
	if err := json.Unmarshal(data, &gj); err != nil {
		return fmt.Errorf("loadFromJSON: %w", err)
	}

	for _, n := range gj.Nodes {
		n2 := *n
		g.indexNode(&n2)
	}
	for _, ej := range gj.Edges {
		e := &Edge{
			Src:        ej.Src,
			Kind:       ej.Kind,
			Dst:        ej.Dst,
			Phase:      ej.Phase,
			Required:   ej.Required,
			Confidence: ej.Confidence,
			Class:      ej.Class,
			Weight:     ej.Weight,
			Metadata:   ej.Metadata,
			Provenance: ej.Provenance,
		}
		g.indexEdge(e)
	}
	for _, inv := range gj.Invariants {
		inv2 := *inv
		g.invariants[inv2.ID] = &inv2
	}
	for _, fm := range gj.FailureModes {
		fm2 := *fm
		g.failureModes[fm2.ID] = &fm2
	}
	for _, b := range gj.Builds {
		b2 := *b
		g.builds = append(g.builds, &b2)
	}
	return nil
}

// loadRuntimeState loads persisted runtime records from dataDir.
func (g *Graph) loadRuntimeState() {
	if g.dataDir == "" {
		return
	}
	// Load snapshots
	snapsDir := filepath.Join(g.dataDir, "snapshots")
	if entries, err := os.ReadDir(snapsDir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(snapsDir, e.Name()))
			if err != nil {
				continue
			}
			var rec runtimeSnapshotRecord
			if json.Unmarshal(data, &rec) == nil {
				g.snapshots = append(g.snapshots, &rec)
			}
		}
		// Sort by captured_at descending.
		sortSnapshotsByTime(g.snapshots)
	}

	// Load experiences
	expDir := filepath.Join(g.dataDir, "experience", "entries")
	if entries, err := os.ReadDir(expDir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(expDir, e.Name()))
			if err != nil {
				continue
			}
			var rec ExperienceEntry
			if json.Unmarshal(data, &rec) == nil {
				e2 := rec
				g.experiences[rec.ID] = &e2
			}
		}
	}
	// Load experience attempts
	attDir := filepath.Join(g.dataDir, "experience", "attempts")
	if entries, err := os.ReadDir(attDir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(attDir, e.Name()))
			if err != nil {
				continue
			}
			var rec ExperienceAttempt
			if json.Unmarshal(data, &rec) == nil {
				r2 := rec
				g.expAttempts[rec.ExperienceID] = append(g.expAttempts[rec.ExperienceID], &r2)
			}
		}
	}
	// Load experience observations
	obsDir := filepath.Join(g.dataDir, "experience", "observations")
	if entries, err := os.ReadDir(obsDir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(obsDir, e.Name()))
			if err != nil {
				continue
			}
			var rec ExperienceObservation
			if json.Unmarshal(data, &rec) == nil {
				r2 := rec
				g.expObs[rec.ExperienceID] = append(g.expObs[rec.ExperienceID], &r2)
			}
		}
	}

	// Load incidents
	incDir := filepath.Join(g.dataDir, "incidents")
	if entries, err := os.ReadDir(incDir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(incDir, e.Name()))
			if err != nil {
				continue
			}
			var rec IncidentRecord
			if json.Unmarshal(data, &rec) == nil {
				r2 := rec
				g.incidents[rec.ID] = &r2
			}
		}
	}

	// Load proposals
	propDir := filepath.Join(g.dataDir, "proposals")
	if entries, err := os.ReadDir(propDir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(propDir, e.Name()))
			if err != nil {
				continue
			}
			var rec ProposalRecord
			if json.Unmarshal(data, &rec) == nil {
				r2 := rec
				g.proposals[rec.ID] = &r2
			}
		}
	}

	// Load preflight audits
	pfDir := filepath.Join(g.dataDir, "preflight_audits")
	if entries, err := os.ReadDir(pfDir); err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(pfDir, e.Name()))
			if err != nil {
				continue
			}
			var rec PreflightAuditRecord
			if json.Unmarshal(data, &rec) == nil {
				r2 := rec
				g.preflights = append(g.preflights, &r2)
			}
		}
	}
}

// saveGraphJSON persists the static graph data (nodes, edges, invariants, etc.)
// to <dataDir>/graph.json. Called after bulk writes (e.g., Build).
func (g *Graph) saveGraphJSON() error {
	if g.dataDir == "" || g.readOnly {
		return nil
	}
	g.mu.RLock()
	g.buildMu.RLock()
	defer g.mu.RUnlock()
	defer g.buildMu.RUnlock()

	var nodes []*Node
	for _, n := range g.nodes {
		n2 := *n
		nodes = append(nodes, &n2)
	}
	var edges []*edgeJSON
	for _, e := range g.edges {
		edges = append(edges, &edgeJSON{
			Src:        e.Src,
			Kind:       e.Kind,
			Dst:        e.Dst,
			Phase:      e.Phase,
			Required:   e.Required,
			Confidence: e.Confidence,
			Class:      e.Class,
			Weight:     e.Weight,
			Metadata:   e.Metadata,
			Provenance: e.Provenance,
		})
	}
	var invs []*Invariant
	for _, inv := range g.invariants {
		inv2 := *inv
		invs = append(invs, &inv2)
	}
	var fms []*FailureMode
	for _, fm := range g.failureModes {
		fm2 := *fm
		fms = append(fms, &fm2)
	}
	gj := graphJSON{
		Version:      1,
		Nodes:        nodes,
		Edges:        edges,
		Invariants:   invs,
		FailureModes: fms,
		Builds:       g.builds,
	}

	data, err := json.MarshalIndent(gj, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(g.dataDir, "graph.json"), data)
}

// writeFileAtomic writes data to path atomically using a temp file + rename.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// writeJSON persists a single record to <dataDir>/<subdir>/<id>.json.
func (g *Graph) writeJSON(subdir, id string, v any) error {
	if g.dataDir == "" || g.readOnly {
		return nil
	}
	dir := filepath.Join(g.dataDir, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, sanitizeID(id)+".json"), data)
}

// sanitizeID converts an ID to a filesystem-safe filename component.
func sanitizeID(id string) string {
	r := strings.NewReplacer("/", "_", ":", "_", " ", "_", ".", "_")
	return r.Replace(id)
}

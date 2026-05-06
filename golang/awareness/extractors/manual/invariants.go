package manual

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/graph"
)

// invariantFile is the top-level structure of invariants.yaml and convergence_rules.yaml.
type invariantFile struct {
	Invariants []yamlInvariant `yaml:"invariants"`
}

type yamlInvariant struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Severity string `yaml:"severity"`
	Status   string `yaml:"status"`
	Summary  string `yaml:"summary"`
	Protects struct {
		State        []string `yaml:"state"`
		Files        []string `yaml:"files"`
		Symbols      []string `yaml:"symbols"`
		SystemdUnits []string `yaml:"systemd_units"`
	} `yaml:"protects"`
	ForbiddenFixes    []string `yaml:"forbidden_fixes"`
	RequiredTests     []string `yaml:"required_tests"`
	RelatedFailureModes []string `yaml:"related_failure_modes"`
}

// LoadInvariants loads invariants.yaml or convergence_rules.yaml into g.
// Missing files are silently skipped.
func LoadInvariants(ctx context.Context, g *graph.Graph, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("LoadInvariants: read %s: %w", path, err)
	}

	var f invariantFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("LoadInvariants: parse %s: %w", path, err)
	}

	for _, inv := range f.Invariants {
		if err := loadInvariant(ctx, g, inv); err != nil {
			return fmt.Errorf("LoadInvariants %s: %w", inv.ID, err)
		}
	}
	return nil
}

func loadInvariant(ctx context.Context, g *graph.Graph, inv yamlInvariant) error {
	nodeID := "invariant:" + inv.ID

	if err := g.AddNode(ctx, graph.Node{
		ID:      nodeID,
		Type:    graph.NodeTypeInvariant,
		Name:    inv.ID,
		Summary: inv.Summary,
	}); err != nil {
		return err
	}

	if err := g.UpsertInvariant(ctx, graph.Invariant{
		ID:       inv.ID,
		Title:    inv.Title,
		Summary:  inv.Summary,
		Severity: inv.Severity,
		Status:   inv.Status,
	}); err != nil {
		return err
	}

	// Protected etcd keys → etcd_key nodes + protects edges.
	for _, state := range inv.Protects.State {
		stateID := "etcd_key:" + state
		if err := g.AddNode(ctx, graph.Node{
			ID:   stateID,
			Type: graph.NodeTypeEtcdKey,
			Name: state,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeProtects, Dst: stateID}); err != nil {
			return err
		}
	}

	// Protected files → source_file nodes + protects edges.
	for _, file := range inv.Protects.Files {
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
	}

	// Protected symbols → symbol nodes + protects edges.
	for _, sym := range inv.Protects.Symbols {
		symID := "symbol:" + sym
		if err := g.AddNode(ctx, graph.Node{
			ID:   symID,
			Type: graph.NodeTypeSymbol,
			Name: sym,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeProtects, Dst: symID}); err != nil {
			return err
		}
	}

	// Protected systemd units → systemd_unit nodes + protects edges.
	for _, unit := range inv.Protects.SystemdUnits {
		unitID := "systemd_unit:" + unit
		if err := g.AddNode(ctx, graph.Node{
			ID:   unitID,
			Type: graph.NodeTypeSystemdUnit,
			Name: unit,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeProtects, Dst: unitID}); err != nil {
			return err
		}
	}

	// Forbidden fixes → forbidden_fix nodes + forbids edges.
	for _, fix := range inv.ForbiddenFixes {
		fixID := "forbidden_fix:" + fix
		if err := g.AddNode(ctx, graph.Node{
			ID:   fixID,
			Type: graph.NodeTypeForbiddenFix,
			Name: fix,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeForbids, Dst: fixID}); err != nil {
			return err
		}
	}

	// Required tests → test nodes + tested_by edges.
	for _, test := range inv.RequiredTests {
		testID := "test:" + test
		if err := g.AddNode(ctx, graph.Node{
			ID:   testID,
			Type: graph.NodeTypeTest,
			Name: test,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeTestedBy, Dst: testID}); err != nil {
			return err
		}
	}

	// Related failure modes → best-effort edges (failure mode nodes may not exist yet).
	for _, fmID := range inv.RelatedFailureModes {
		fmNodeID := "failure_mode:" + fmID
		// Ensure the node exists so the edge target is valid.
		if err := g.AddNode(ctx, graph.Node{
			ID:   fmNodeID,
			Type: graph.NodeTypeFailureMode,
			Name: fmID,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeAffects, Dst: fmNodeID}); err != nil {
			return err
		}
	}

	return nil
}

package manual

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/graph"
)

// failureModeFile is the top-level structure of failure_modes.yaml.
type failureModeFile struct {
	FailureModes []yamlFailureMode `yaml:"failure_modes"`
}

type yamlFailureMode struct {
	ID              string   `yaml:"id"`
	Title           string   `yaml:"title"`
	Severity        string   `yaml:"severity"`
	Symptoms        []string `yaml:"symptoms"`
	RootCause       string   `yaml:"root_cause"`
	ArchitectureFix string   `yaml:"architecture_fix"`
	ForbiddenFixes  []string `yaml:"forbidden_fixes"`
	RelatedInvariants []string `yaml:"related_invariants"`
	RelatedServices   []string `yaml:"related_services"`
	RequiredTests     []string `yaml:"required_tests"`
}

// LoadFailureModes loads failure_modes.yaml into g.
// Missing files are silently skipped.
func LoadFailureModes(ctx context.Context, g *graph.Graph, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("LoadFailureModes: read %s: %w", path, err)
	}

	var f failureModeFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("LoadFailureModes: parse %s: %w", path, err)
	}

	for _, fm := range f.FailureModes {
		if err := loadFailureMode(ctx, g, fm); err != nil {
			return fmt.Errorf("LoadFailureModes %s: %w", fm.ID, err)
		}
	}
	return nil
}

func loadFailureMode(ctx context.Context, g *graph.Graph, fm yamlFailureMode) error {
	nodeID := "failure_mode:" + fm.ID

	if err := g.AddNode(ctx, graph.Node{
		ID:      nodeID,
		Type:    graph.NodeTypeFailureMode,
		Name:    fm.ID,
		Summary: fm.RootCause,
	}); err != nil {
		return err
	}

	if err := g.UpsertFailureMode(ctx, graph.FailureMode{
		ID:              fm.ID,
		Title:           fm.Title,
		Summary:         fm.RootCause,
		Symptoms:        fm.Symptoms,
		RootCause:       fm.RootCause,
		ArchitectureFix: fm.ArchitectureFix,
	}); err != nil {
		return err
	}

	// Forbidden fixes → forbidden_fix nodes + forbids edges.
	for _, fix := range fm.ForbiddenFixes {
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

	// Related invariants → violates edges (invariant nodes may already exist).
	for _, invID := range fm.RelatedInvariants {
		invNodeID := "invariant:" + invID
		if err := g.AddNode(ctx, graph.Node{
			ID:   invNodeID,
			Type: graph.NodeTypeInvariant,
			Name: invID,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeViolates, Dst: invNodeID}); err != nil {
			return err
		}
	}

	// Related services → globular_service nodes + affects edges.
	for _, svc := range fm.RelatedServices {
		svcID := "service:" + svc
		if err := g.AddNode(ctx, graph.Node{
			ID:   svcID,
			Type: graph.NodeTypeGlobularService,
			Name: svc,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: nodeID, Kind: graph.EdgeAffects, Dst: svcID}); err != nil {
			return err
		}
	}

	// Required tests → test nodes + tested_by edges.
	for _, test := range fm.RequiredTests {
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

	return nil
}

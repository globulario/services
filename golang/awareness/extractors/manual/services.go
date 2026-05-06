package manual

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/graph"
)

// servicesFile is the top-level structure of services.yaml.
type servicesFile struct {
	Services []yamlService `yaml:"services"`
}

type yamlService struct {
	ID           string             `yaml:"id"`
	Name         string             `yaml:"name"`
	Summary      string             `yaml:"summary"`
	SystemdUnit  string             `yaml:"systemd_unit"`
	ProtoService string             `yaml:"proto_service"`
	DependsOn    []yamlDependency   `yaml:"depends_on"`
}

type yamlDependency struct {
	Service  string `yaml:"service"`
	Phase    string `yaml:"phase"`
	Required bool   `yaml:"required"`
}

// LoadServices loads services.yaml into g.
// Missing files are silently skipped.
func LoadServices(ctx context.Context, g *graph.Graph, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("LoadServices: read %s: %w", path, err)
	}

	var f servicesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("LoadServices: parse %s: %w", path, err)
	}

	for _, svc := range f.Services {
		if err := loadService(ctx, g, svc); err != nil {
			return fmt.Errorf("LoadServices %s: %w", svc.ID, err)
		}
	}
	return nil
}

func loadService(ctx context.Context, g *graph.Graph, svc yamlService) error {
	svcID := "service:" + svc.ID
	name := svc.Name
	if name == "" {
		name = svc.ID
	}

	if err := g.AddNode(ctx, graph.Node{
		ID:      svcID,
		Type:    graph.NodeTypeGlobularService,
		Name:    name,
		Summary: svc.Summary,
	}); err != nil {
		return err
	}

	// Systemd unit → systemd_unit node + runs_as edge.
	if svc.SystemdUnit != "" {
		unitID := "systemd_unit:" + svc.SystemdUnit
		if err := g.AddNode(ctx, graph.Node{
			ID:   unitID,
			Type: graph.NodeTypeSystemdUnit,
			Name: svc.SystemdUnit,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{
			Src:  svcID,
			Kind: graph.EdgeRunsAs,
			Dst:  unitID,
		}); err != nil {
			return err
		}
	}

	// Proto service → proto_service node + owns edge.
	if svc.ProtoService != "" {
		protoID := "proto_service:" + svc.ProtoService
		if err := g.AddNode(ctx, graph.Node{
			ID:   protoID,
			Type: graph.NodeTypeProtoService,
			Name: svc.ProtoService,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{
			Src:  svcID,
			Kind: graph.EdgeOwns,
			Dst:  protoID,
		}); err != nil {
			return err
		}
	}

	// Dependencies → ensure target node exists + depends_on edge.
	for _, dep := range svc.DependsOn {
		dstID := "service:" + dep.Service
		if err := g.AddNode(ctx, graph.Node{
			ID:   dstID,
			Type: graph.NodeTypeGlobularService,
			Name: dep.Service,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{
			Src:      svcID,
			Kind:     graph.EdgeDependsOn,
			Dst:      dstID,
			Phase:    dep.Phase,
			Required: dep.Required,
		}); err != nil {
			return err
		}
	}

	return nil
}

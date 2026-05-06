// Package packages extracts package nodes from Globular package.json manifests
// and release-index.json files.
package packages

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

// packageManifest mirrors the relevant fields of a Globular package.json.
type packageManifest struct {
	Name         string   `json:"name"`
	ServiceName  string   `json:"service_name"`
	Version      string   `json:"version"`
	Kind         string   `json:"kind"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies"`
}

// releaseIndex mirrors the top level of release-index.json.
type releaseIndex struct {
	Packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Kind    string `json:"kind"`
	} `json:"packages"`
}

// Extract walks repoRoot for package.json files and the release-index.json.
func Extract(ctx context.Context, g *graph.Graph, repoRoot string) error {
	// Walk for package.json manifests.
	if err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "package.json" {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		return extractPackageManifest(ctx, g, path, rel)
	}); err != nil {
		return err
	}

	// Try release-index.json in common locations.
	for _, candidate := range []string{
		filepath.Join(repoRoot, "release-index.json"),
		filepath.Join(repoRoot, "packages", "release-index.json"),
	} {
		if data, err := os.ReadFile(candidate); err == nil {
			rel, _ := filepath.Rel(repoRoot, candidate)
			_ = extractReleaseIndex(ctx, g, data, rel)
			break
		}
	}

	return nil
}

func extractPackageManifest(ctx context.Context, g *graph.Graph, absPath, relPath string) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}

	var m packageManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if m.Name == "" {
		return nil
	}

	pkgID := "package:" + m.Name
	if err := g.AddNode(ctx, graph.Node{
		ID:      pkgID,
		Type:    graph.NodeTypePackage,
		Name:    m.Name,
		Path:    relPath,
		Summary: m.Description,
		Metadata: map[string]any{
			"version": m.Version,
			"kind":    m.Kind,
		},
	}); err != nil {
		return err
	}

	// Link package to its globular service.
	if m.ServiceName != "" {
		svcID := "service:" + m.ServiceName
		_ = g.AddNode(ctx, graph.Node{ID: svcID, Type: graph.NodeTypeGlobularService, Name: m.ServiceName})
		_ = g.AddEdge(ctx, graph.Edge{Src: pkgID, Kind: graph.EdgeOwns, Dst: svcID})
	}

	// Dependencies.
	for _, dep := range m.Dependencies {
		depID := "package:" + dep
		_ = g.AddNode(ctx, graph.Node{ID: depID, Type: graph.NodeTypePackage, Name: dep})
		_ = g.AddEdge(ctx, graph.Edge{Src: pkgID, Kind: graph.EdgeDependsOn, Dst: depID})
	}

	return nil
}

func extractReleaseIndex(ctx context.Context, g *graph.Graph, data []byte, relPath string) error {
	var idx releaseIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil
	}
	for _, p := range idx.Packages {
		if p.Name == "" {
			continue
		}
		pkgID := "package:" + p.Name
		_ = g.AddNode(ctx, graph.Node{
			ID:   pkgID,
			Type: graph.NodeTypePackage,
			Name: p.Name,
			Path: relPath,
			Metadata: map[string]any{
				"version": p.Version,
				"kind":    p.Kind,
			},
		})
	}
	return nil
}

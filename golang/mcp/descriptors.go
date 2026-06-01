package main

import (
	"context"
	"fmt"
	"time"

	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// ServiceDescriptor is a normalized view of a service's dependency and
// capability metadata, derived from its ArtifactManifest in the repository.
type ServiceDescriptor struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Requires []string `json:"requires,omitempty"` // service names this depends on
	Provides []string `json:"provides,omitempty"` // capabilities this service offers
}

// DescriptorLookup can retrieve a ServiceDescriptor from package metadata.
// This is an optional capability — sources that don't support it return an error.
type DescriptorLookup interface {
	// Descriptor fetches the descriptor for a service from the repository.
	// Returns ErrNoDescriptor if the source doesn't support descriptor lookup.
	Descriptor(ctx context.Context, service string) (*ServiceDescriptor, error)
}

// ErrNoDescriptor is returned when a DependencySource does not support
// descriptor lookup (e.g. the static bootstrap source).
var ErrNoDescriptor = fmt.Errorf("descriptor lookup not supported")

// ── Repository-backed descriptor source ──────────────────────────────────────

// repoDescriptorSource fetches ServiceDescriptors from the repository
// via GetArtifactManifest, reading the requires/provides fields.
type repoDescriptorSource struct {
	clients *clientPool
}

// NewRepoDescriptorSource creates a DescriptorLookup backed by the package repository.
func NewRepoDescriptorSource(clients *clientPool) DescriptorLookup {
	return &repoDescriptorSource{clients: clients}
}

func (r *repoDescriptorSource) Descriptor(ctx context.Context, service string) (*ServiceDescriptor, error) {
	if r.clients == nil {
		return nil, fmt.Errorf("no client pool available")
	}

	queryCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
	defer cancel()

	conn, err := r.clients.get(queryCtx, repositoryEndpoint())
	if err != nil {
		return nil, fmt.Errorf("repository unavailable: %w", err)
	}

	client := repositorypb.NewPackageRepositoryClient(conn)

	// First, find the artifact to get publisher/version/platform.
	// We search by name and take the first match.
	searchResp, err := client.SearchArtifacts(queryCtx, &repositorypb.SearchArtifactsRequest{
		Query:    service,
		PageSize: 5,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Find exact match
	var matchRef *repositorypb.ArtifactRef
	for _, a := range searchResp.GetArtifacts() {
		if a.GetRef() != nil && a.GetRef().GetName() == service {
			matchRef = a.GetRef()
			break
		}
	}
	if matchRef == nil {
		return nil, fmt.Errorf("artifact %q not found in repository", service)
	}

	// Now get the full manifest with requires/provides
	manifestResp, err := client.GetArtifactManifest(queryCtx, &repositorypb.GetArtifactManifestRequest{
		Ref: matchRef,
	})
	if err != nil {
		return nil, fmt.Errorf("GetArtifactManifest: %w", err)
	}

	manifest := manifestResp.GetManifest()
	if manifest == nil {
		return nil, fmt.Errorf("manifest is nil for %q", service)
	}

	desc := &ServiceDescriptor{
		Name:     service,
		Version:  matchRef.GetVersion(),
		Requires: manifest.GetRequires(),
		Provides: manifest.GetProvides(),
	}

	return desc, nil
}

// ── Transitive dependency resolution ─────────────────────────────────────────

const maxTransitiveDepth = 10

// ResolveTransitiveDeps performs a BFS over the descriptor graph to find
// all transitive dependencies of a service. Returns the ordered list of
// all required services and whether a cycle was detected.
//
// The lookup function is called for each service to get its descriptor.
// If lookup returns an error for a service, that service is treated as
// a leaf (no further dependencies explored).
func ResolveTransitiveDeps(root string, lookup func(string) (*ServiceDescriptor, error)) (allDeps []string, cycle bool) {
	visited := map[string]bool{root: true}
	queue := []string{root}
	depth := map[string]int{root: 0}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if depth[current] >= maxTransitiveDepth {
			cycle = true
			break
		}

		desc, err := lookup(current)
		if err != nil || desc == nil {
			continue
		}

		for _, req := range desc.Requires {
			if req == root {
				cycle = true
				continue
			}
			if !visited[req] {
				visited[req] = true
				allDeps = append(allDeps, req)
				queue = append(queue, req)
				depth[req] = depth[current] + 1
			}
		}
	}

	return allDeps, cycle
}

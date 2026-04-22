package main

import (
	"strings"

	"github.com/globulario/services/golang/config"
)

// ── Dependency Source ────────────────────────────────────────────────────────

// ServiceDependency describes a single dependency of a service.
type ServiceDependency struct {
	Name              string `json:"name"`
	Required          bool   `json:"required"`
	VersionConstraint string `json:"version_constraint,omitempty"` // empty = any version
}

// DependencySource abstracts where service dependency metadata comes from.
// The static implementation is the bootstrap; later this can be swapped
// to read from package/service descriptors without changing the planner.
type DependencySource interface {
	// Dependencies returns the services that the named service requires.
	Dependencies(service string) []ServiceDependency
	// DefaultPorts returns the ports a service is known to use.
	DefaultPorts(service string) []int
	// ReverseDeps returns services that depend on the given service.
	// The installed list scopes the lookup to only currently running services.
	ReverseDeps(service string, installed []string) []string
}

// ── Static (bootstrap) implementation ────────────────────────────────────────

type staticDeps struct {
	deps map[string][]ServiceDependency
}

// NewStaticDependencySource returns a DependencySource backed by the dependency
// graph for known Globular services. Ports are resolved from etcd at runtime.
func NewStaticDependencySource() DependencySource {
	return &staticDeps{
		deps: map[string][]ServiceDependency{
			// Core infrastructure — no service dependencies
			"event":      nil,
			"discovery":  nil,
			"dns":        nil,
			"monitoring": nil,

			// Auth / identity
			"authentication": {
				{Name: "persistence", Required: true},
				{Name: "event", Required: false},
			},
			"rbac": {
				{Name: "persistence", Required: true},
				{Name: "resource", Required: true},
			},
			"resource": {
				{Name: "persistence", Required: true},
			},

			// Data services
			"persistence": nil, // external backends (mongo/scylla)
			"storage":     nil,
			"sql":         nil,

			// Application services
			"ldap": {
				{Name: "persistence", Required: true},
				{Name: "authentication", Required: true},
				{Name: "rbac", Required: false},
			},
			"blog": {
				{Name: "persistence", Required: true},
				{Name: "file", Required: true},
			},
			"file": {
				{Name: "rbac", Required: true},
				{Name: "resource", Required: true},
			},
			"mail": {
				{Name: "persistence", Required: true},
			},
			"media": {
				{Name: "file", Required: true},
				{Name: "persistence", Required: true},
			},
			"title": {
				{Name: "persistence", Required: true},
			},
			"conversation": {
				{Name: "persistence", Required: true},
			},
			"search": {
				{Name: "persistence", Required: true},
			},
			"catalog": {
				{Name: "persistence", Required: true},
			},
			"repository": {
				{Name: "persistence", Required: true},
				{Name: "storage", Required: true},
			},
			"log": {
				{Name: "persistence", Required: true},
			},
			"spc": {
				{Name: "persistence", Required: true},
			},
			"torrent": nil,

			// AI services
			"ai_memory": {
				{Name: "persistence", Required: true}, // ScyllaDB
			},
			"ai_watcher": {
				{Name: "event", Required: true},
				{Name: "ai_memory", Required: true},
			},
		},
	}
}

func (s *staticDeps) Dependencies(service string) []ServiceDependency {
	return s.deps[service]
}

// DefaultPorts resolves the ports for a named service from etcd.
// etcd is the sole source of truth — no hardcoded port constants.
// Returns nil if the service is not registered or etcd is unreachable.
func (s *staticDeps) DefaultPorts(service string) []int {
	all, err := config.GetServicesConfigurations()
	if err != nil {
		return nil
	}
	var ports []int
	for _, sc := range all {
		name, _ := sc["Name"].(string)
		// Match services whose Name or Id contains the short service name.
		if !strings.Contains(strings.ToLower(name), strings.ToLower(service)) {
			continue
		}
		switch p := sc["Port"].(type) {
		case float64:
			if int(p) > 0 {
				ports = append(ports, int(p))
			}
		case int:
			if p > 0 {
				ports = append(ports, p)
			}
		}
	}
	return ports
}

// ReverseDeps inverts the dependency graph: returns installed services that
// depend on the given service (i.e. have it in their requires list).
func (s *staticDeps) ReverseDeps(service string, installed []string) []string {
	installedSet := make(map[string]bool, len(installed))
	for _, svc := range installed {
		installedSet[svc] = true
	}

	var dependents []string
	for svc, deps := range s.deps {
		if !installedSet[svc] {
			continue
		}
		for _, d := range deps {
			if d.Name == service && d.Required {
				dependents = append(dependents, svc)
				break
			}
		}
	}
	return dependents
}

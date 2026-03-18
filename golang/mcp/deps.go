package main

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
	deps  map[string][]ServiceDependency
	ports map[string][]int
}

// NewStaticDependencySource returns a DependencySource backed by a hard-coded
// map of known Globular services. This is the bootstrap implementation; the
// interface allows swapping to descriptor-backed metadata later.
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
		},

		ports: map[string][]int{
			"authentication": {10101},
			"event":          {10102},
			"file":           {10103},
			"rbac":           {10104},
			"dns":            {10006},
			"persistence":    {10201},
			"resource":       {10301},
			"discovery":      {10401},
			"monitoring":     {10501},
			"ldap":           {10601},
			"blog":           {10701},
			"mail":           {10801},
			"media":          {10901},
			"title":          {11001},
			"conversation":   {11101},
			"search":         {11201},
			"storage":        {11301},
			"catalog":        {11401},
			"repository":     {11501},
			"log":            {11601},
			"spc":            {11701},
			"torrent":        {11801},
			"sql":            {11901},
		},
	}
}

func (s *staticDeps) Dependencies(service string) []ServiceDependency {
	return s.deps[service]
}

func (s *staticDeps) DefaultPorts(service string) []int {
	return s.ports[service]
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

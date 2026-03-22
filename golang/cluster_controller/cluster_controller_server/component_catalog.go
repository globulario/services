package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Capability model
// ---------------------------------------------------------------------------

// Capability represents something a package provides to the node.
type Capability string

const (
	CapConfigStore      Capability = "config-store"
	CapDNS              Capability = "dns"
	CapServiceDiscovery Capability = "service-discovery"
	CapEventBus         Capability = "event-bus"
	CapObjectStore      Capability = "object-store"
	CapLocalDB          Capability = "local-db"
	CapHTTPProxy        Capability = "http-proxy"
	CapServiceMesh      Capability = "service-mesh"
	CapGateway          Capability = "gateway"
	CapMonitoring       Capability = "monitoring"
)

// ProfileCapabilities maps each profile to the capabilities it requires.
// A capability triggers installation of the infra component(s) that provide it.
var ProfileCapabilities = map[string][]Capability{
	"core":          {CapConfigStore, CapDNS, CapServiceDiscovery, CapEventBus, CapObjectStore, CapMonitoring},
	"compute":       {CapConfigStore, CapDNS, CapServiceDiscovery, CapEventBus, CapObjectStore, CapMonitoring},
	"control-plane": {CapConfigStore, CapDNS, CapServiceDiscovery},
	"gateway":       {CapHTTPProxy, CapServiceMesh, CapGateway},
	"storage":       {CapObjectStore},
	"dns":           {CapDNS},
	"scylla":        {CapLocalDB},
	"database":      {CapLocalDB},
}

// ---------------------------------------------------------------------------
// Component model
// ---------------------------------------------------------------------------

// ComponentKind classifies components for tier gating.
type ComponentKind int

const (
	KindInfrastructure ComponentKind = iota
	KindWorkload
)

// HealthCheckHintC describes how to verify a component is healthy on a node.
// (Suffixed with C to avoid collision with pkgpack.HealthCheckHint.)
type HealthCheckHintC struct {
	Unit string // systemd unit that must be active
	Port int    // TCP port that must be listening (0 = skip)
}

// Component is a single deployable unit in the cluster catalog.
type Component struct {
	// Name is the canonical kebab-case key (e.g. "etcd", "ai-memory").
	Name string

	// Unit is the systemd unit name (e.g. "globular-etcd.service").
	Unit string

	// Kind classifies the component for tier gating.
	Kind ComponentKind

	// Priority determines start order (lower = starts first, stops last).
	Priority int

	// Profiles lists which profiles include this component.
	Profiles []string

	// ProvidesCapabilities lists what this component gives the node.
	ProvidesCapabilities []Capability

	// InstallDependencies lists packages that must be installed before this one.
	InstallDependencies []string

	// RuntimeLocalDependencies lists packages that must be healthy on the
	// same node before this component can start.
	RuntimeLocalDependencies []string

	// ManagedUnit means this component is included in profileUnitMap for
	// unit enable/start/stop/disable actions, even if Kind is KindWorkload.
	// This matches the old behavior where event/rbac/file were in
	// profileUnitMap but not in unitTier as infrastructure.
	ManagedUnit bool

	// HealthCheck describes how to verify this component is healthy.
	HealthCheck *HealthCheckHintC
}

// ---------------------------------------------------------------------------
// Catalog registry
// ---------------------------------------------------------------------------

// catalog is the authoritative list of all known components.
// Infrastructure components first, then workloads, each sorted by priority.
var catalog []*Component

// catalogIndex maps canonical component name → *Component for O(1) lookup.
var catalogIndex map[string]*Component

// catalogByUnit maps systemd unit name → *Component for O(1) lookup.
var catalogByUnit map[string]*Component

func init() {
	catalog = buildCatalog()

	// Build indexes.
	catalogIndex = make(map[string]*Component, len(catalog))
	catalogByUnit = make(map[string]*Component, len(catalog))
	for _, c := range catalog {
		catalogIndex[c.Name] = c
		catalogByUnit[strings.ToLower(c.Unit)] = c
	}

	// Derive backward-compat maps used by plan.go, service_config.go, bootstrap_phases.go.
	rebuildDerivedMaps()
}

func buildCatalog() []*Component {
	return []*Component{
		// ---------------------------------------------------------------
		// Infrastructure components
		// ---------------------------------------------------------------
		{
			Name:                 "etcd",
			Unit:                 "globular-etcd.service",
			Kind:                 KindInfrastructure,
			Priority:             1,
			Profiles:             []string{"core", "compute", "control-plane"},
			ProvidesCapabilities: []Capability{CapConfigStore},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-etcd.service", Port: 2379},
		},
		{
			Name:                 "dns",
			Unit:                 "globular-dns.service",
			Kind:                 KindInfrastructure,
			Priority:             2,
			Profiles:             []string{"core", "compute", "control-plane", "dns"},
			ProvidesCapabilities: []Capability{CapDNS},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-dns.service", Port: 10006},
		},
		{
			Name:                 "discovery",
			Unit:                 "globular-discovery.service",
			Kind:                 KindInfrastructure,
			Priority:             3,
			Profiles:             []string{"core", "compute", "control-plane"},
			ProvidesCapabilities: []Capability{CapServiceDiscovery},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-discovery.service"},
		},
		{
			Name:                 "event",
			Unit:                 "globular-event.service",
			Kind:                 KindWorkload,
			Priority:             4,
			Profiles:             []string{"core", "compute"},
			ProvidesCapabilities: []Capability{CapEventBus},
			ManagedUnit:          true, // included in profileUnitMap for unit actions
			HealthCheck:          &HealthCheckHintC{Unit: "globular-event.service"},
		},
		{
			Name:                     "rbac",
			Unit:                     "globular-rbac.service",
			Kind:                     KindWorkload,
			Priority:                 5,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
			ManagedUnit:              true, // included in profileUnitMap for unit actions
			HealthCheck:              &HealthCheckHintC{Unit: "globular-rbac.service"},
		},
		{
			Name:                 "minio",
			Unit:                 "globular-minio.service",
			Kind:                 KindInfrastructure,
			Priority:             6,
			Profiles:             []string{"core", "compute", "storage"},
			ProvidesCapabilities: []Capability{CapObjectStore},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-minio.service", Port: 9000},
		},
		{
			Name:                 "scylladb",
			Unit:                 "scylla-server.service",
			Kind:                 KindInfrastructure,
			Priority:             6,
			Profiles:             []string{"scylla", "database"},
			ProvidesCapabilities: []Capability{CapLocalDB},
			HealthCheck:          &HealthCheckHintC{Unit: "scylla-server.service", Port: 9042},
		},
		{
			Name:        "file",
			Unit:        "globular-file.service",
			Kind:        KindWorkload,
			Priority:    7,
			Profiles:    []string{"core", "compute", "storage"},
			ManagedUnit: true, // included in profileUnitMap for unit actions
			RuntimeLocalDependencies: []string{"event"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-file.service"},
		},
		{
			Name:                 "monitoring",
			Unit:                 "globular-monitoring.service",
			Kind:                 KindInfrastructure,
			Priority:             8,
			Profiles:             []string{"core", "compute"},
			ProvidesCapabilities: []Capability{CapMonitoring},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-monitoring.service"},
		},
		{
			Name:                 "xds",
			Unit:                 "globular-xds.service",
			Kind:                 KindInfrastructure,
			Priority:             9,
			Profiles:             []string{"core", "compute", "control-plane", "gateway"},
			ProvidesCapabilities: []Capability{CapServiceMesh},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-xds.service"},
		},
		{
			Name:                 "gateway",
			Unit:                 "globular-gateway.service",
			Kind:                 KindInfrastructure,
			Priority:             9,
			Profiles:             []string{"gateway"},
			ProvidesCapabilities: []Capability{CapGateway},
			RuntimeLocalDependencies: []string{"xds", "envoy"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-gateway.service", Port: 8080},
		},
		{
			Name:                 "envoy",
			Unit:                 "globular-envoy.service",
			Kind:                 KindInfrastructure,
			Priority:             10,
			Profiles:             []string{"gateway"},
			ProvidesCapabilities: []Capability{CapHTTPProxy},
			RuntimeLocalDependencies: []string{"xds"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-envoy.service", Port: 8443},
		},

		// ---------------------------------------------------------------
		// Workload components
		// ---------------------------------------------------------------
		{
			Name:                     "authentication",
			Unit:                     "globular-authentication.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event", "rbac"},
		},
		{
			Name:                     "resource",
			Unit:                     "globular-resource.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "persistence",
			Unit:                     "globular-persistence.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "sql",
			Unit:                     "globular-sql.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "storage",
			Unit:                     "globular-storage.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute", "storage"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "repository",
			Unit:                     "globular-repository.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "catalog",
			Unit:                     "globular-catalog.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event", "persistence"},
		},
		{
			Name:                     "search",
			Unit:                     "globular-search.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "log",
			Unit:                     "globular-log.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "ldap",
			Unit:                     "globular-ldap.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "mail",
			Unit:                     "globular-mail.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "blog",
			Unit:                     "globular-blog.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "conversation",
			Unit:                     "globular-conversation.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "title",
			Unit:                     "globular-title.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "media",
			Unit:                     "globular-media.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "torrent",
			Unit:                     "globular-torrent.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "echo",
			Unit:                     "globular-echo.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "backup-manager",
			Unit:                     "globular-backup-manager.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute", "storage"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "cluster-controller",
			Unit:                     "globular-cluster-controller.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"control-plane"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "cluster-doctor",
			Unit:                     "globular-cluster-doctor.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"control-plane"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "ai-memory",
			Unit:                     "globular-ai-memory.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"scylla", "database"},
			RuntimeLocalDependencies: []string{"scylladb", "event"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-ai-memory.service", Port: 10200},
		},
		{
			Name:                     "ai-executor",
			Unit:                     "globular-ai-executor.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"scylla", "database"},
			RuntimeLocalDependencies: []string{"ai-memory", "event"},
		},
		{
			Name:                     "ai-router",
			Unit:                     "globular-ai-router.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
		{
			Name:                     "ai-watcher",
			Unit:                     "globular-ai-watcher.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"scylla", "database"},
			RuntimeLocalDependencies: []string{"ai-executor", "event"},
		},
		{
			Name:                     "mcp",
			Unit:                     "globular-mcp.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},
	}
}

// ---------------------------------------------------------------------------
// Derived maps — backward compat for plan.go, service_config.go, etc.
// ---------------------------------------------------------------------------

// rebuildDerivedMaps populates the package-level maps that existing code
// depends on (profileUnitMap, unitTier, unitPriority, allManagedUnits,
// profile-for-X vars). Called once from init().
func rebuildDerivedMaps() {
	// profileUnitMap: infrastructure components + ManagedUnit workloads.
	// The old map contained infra + some workloads (event, rbac, file).
	newProfileMap := make(map[string][]string)
	for _, c := range catalog {
		if c.Kind != KindInfrastructure && !c.ManagedUnit {
			continue
		}
		for _, p := range c.Profiles {
			newProfileMap[p] = appendUniqueStr(newProfileMap[p], c.Unit)
		}
	}
	profileUnitMap = newProfileMap

	// unitTier
	newTier := make(map[string]ServiceTier)
	for _, c := range catalog {
		if c.Kind == KindInfrastructure {
			newTier[strings.ToLower(c.Unit)] = TierInfrastructure
		}
		// KindWorkload defaults to TierWorkload via getUnitTier fallback.
	}
	unitTier = newTier

	// unitPriority
	newPriority := make(map[string]int)
	for _, c := range catalog {
		newPriority[strings.ToLower(c.Unit)] = c.Priority
	}
	unitPriority = newPriority

	// allManagedUnits (infra only — matches old behavior)
	seen := make(map[string]struct{})
	for _, units := range profileUnitMap {
		for _, u := range units {
			seen[strings.ToLower(u)] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for u := range seen {
		result = append(result, u)
	}
	sort.Strings(result)
	allManagedUnits = result
}

func appendUniqueStr(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// ---------------------------------------------------------------------------
// Catalog query helpers
// ---------------------------------------------------------------------------

// CatalogByName returns the component with the given canonical name, or nil.
func CatalogByName(name string) *Component {
	return catalogIndex[name]
}

// CatalogByUnitName returns the component for a systemd unit name, or nil.
func CatalogByUnitName(unit string) *Component {
	return catalogByUnit[strings.ToLower(unit)]
}

// ComponentsForProfile returns all components that belong to a profile.
func ComponentsForProfile(profile string) []*Component {
	var out []*Component
	for _, c := range catalog {
		for _, p := range c.Profiles {
			if p == profile {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// ComponentsProvidingCapability returns infra components that provide cap.
func ComponentsProvidingCapability(cap Capability) []*Component {
	var out []*Component
	for _, c := range catalog {
		for _, provided := range c.ProvidesCapabilities {
			if provided == cap {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// ProfilesForComponent returns the profiles that include a named component.
func ProfilesForComponent(name string) []string {
	c := CatalogByName(name)
	if c == nil {
		return nil
	}
	return append([]string(nil), c.Profiles...)
}

// AllComponents returns a copy of the full catalog.
func AllComponents() []*Component {
	out := make([]*Component, len(catalog))
	copy(out, catalog)
	return out
}

// ValidateProfile returns an error if the profile is unknown.
func ValidateProfile(profile string) error {
	if _, ok := ProfileCapabilities[profile]; ok {
		return nil
	}
	// Also check if any component lists this profile.
	for _, c := range catalog {
		for _, p := range c.Profiles {
			if p == profile {
				return nil
			}
		}
	}
	return fmt.Errorf("unknown profile: %q", profile)
}

// ValidateCatalog checks the catalog for internal consistency:
// no duplicate names/units, all dep references resolve, no cycles.
func ValidateCatalog() error {
	names := make(map[string]bool)
	units := make(map[string]bool)
	for _, c := range catalog {
		if names[c.Name] {
			return fmt.Errorf("duplicate component name: %q", c.Name)
		}
		names[c.Name] = true
		unitLower := strings.ToLower(c.Unit)
		if units[unitLower] {
			return fmt.Errorf("duplicate unit: %q", c.Unit)
		}
		units[unitLower] = true
	}

	// Check dependency references.
	for _, c := range catalog {
		for _, dep := range c.InstallDependencies {
			if !names[dep] {
				return fmt.Errorf("component %q: install dependency %q not in catalog", c.Name, dep)
			}
		}
		for _, dep := range c.RuntimeLocalDependencies {
			if !names[dep] {
				return fmt.Errorf("component %q: runtime dependency %q not in catalog", c.Name, dep)
			}
		}
	}

	// Check for cycles in runtime deps.
	for _, c := range catalog {
		if err := checkCycle(c.Name, nil); err != nil {
			return err
		}
	}

	return nil
}

// checkCycle does DFS cycle detection on RuntimeLocalDependencies.
func checkCycle(name string, path []string) error {
	for _, visited := range path {
		if visited == name {
			return fmt.Errorf("dependency cycle: %s → %s", strings.Join(path, " → "), name)
		}
	}
	c := CatalogByName(name)
	if c == nil {
		return nil
	}
	newPath := append(append([]string(nil), path...), name)
	for _, dep := range c.RuntimeLocalDependencies {
		if err := checkCycle(dep, newPath); err != nil {
			return err
		}
	}
	return nil
}

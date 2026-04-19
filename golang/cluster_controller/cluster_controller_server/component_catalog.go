package main

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
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
	// core provides foundational infra: etcd, dns, event, file, minio, monitoring.
	// ScyllaDB (local-db) is NOT in "core" — it lives in control-plane/storage/scylla/database.
	"core":          {CapConfigStore, CapDNS, CapServiceDiscovery, CapEventBus, CapObjectStore, CapMonitoring},
	"compute":       {CapConfigStore, CapDNS, CapServiceDiscovery, CapEventBus, CapObjectStore, CapMonitoring},
	// control-plane extends core and adds xds/envoy/gateway + local-db (ScyllaDB).
	"control-plane": {CapConfigStore, CapDNS, CapServiceDiscovery, CapEventBus, CapObjectStore, CapMonitoring, CapLocalDB, CapHTTPProxy, CapServiceMesh, CapGateway},
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
	KindCommand // CLI tools (rclone, restic, sctool, etc.) — no systemd unit
)

// HealthCheckHintC describes how to verify a component is healthy on a node.
// (Suffixed with C to avoid collision with pkgpack.HealthCheckHint.)
type HealthCheckHintC struct {
	Unit string // systemd unit that must be active
	Port int    // TCP port that must be listening (0 = skip)
}

// InstallMode constants describe how an infrastructure component is installed.
const (
	// InstallModeRepository means the component is installed from the
	// artifact repository via the standard plan/artifact pipeline.
	InstallModeRepository = "repository"

	// InstallModeDay0Join means the component is installed by the Day 0
	// installer or the Day 1 join state machine (e.g. etcd member-add).
	// The controller should NOT create InfrastructureRelease objects for
	// these — they are managed by dedicated bootstrap/join logic.
	InstallModeDay0Join = "day0_join"
)

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

	// InstallMode describes how this component is installed on nodes.
	// "repository" (default) = installed from artifact repository.
	// "day0_join" = installed by Day 0 installer or Day 1 join logic.
	InstallMode string

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

// ServicesForProfiles returns the set of service names assigned to any of the given profiles.
func ServicesForProfiles(profiles []string) map[string]bool {
	result := make(map[string]bool)
	profileSet := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		profileSet[p] = true
	}
	for _, comp := range catalog {
		if comp.Kind != KindWorkload {
			continue
		}
		for _, p := range comp.Profiles {
			if profileSet[p] {
				result[comp.Name] = true
				break
			}
		}
	}
	return result
}

// RuntimeDependenciesFor returns the RuntimeLocalDependencies for a service name.
func RuntimeDependenciesFor(serviceName string) []string {
	if comp, ok := catalogIndex[serviceName]; ok {
		return comp.RuntimeLocalDependencies
	}
	return nil
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
			InstallMode:          InstallModeDay0Join,
			HealthCheck:          &HealthCheckHintC{Unit: "globular-etcd.service", Port: 2379},
		},
		{
			Name:                 "dns",
			Unit:                 "globular-dns.service",
			Kind:                 KindWorkload,
			Priority:             2,
			Profiles:             []string{"core", "compute", "control-plane", "dns"},
			ProvidesCapabilities: []Capability{CapDNS},
			ManagedUnit:          true,
			HealthCheck:          &HealthCheckHintC{Unit: "globular-dns.service", Port: 10006},
		},
		{
			Name:                 "discovery",
			Unit:                 "globular-discovery.service",
			Kind:                 KindWorkload,
			Priority:             3,
			Profiles:             []string{"core", "compute", "control-plane"},
			ProvidesCapabilities: []Capability{CapServiceDiscovery},
			ManagedUnit:          true,
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
			Profiles:             []string{"core", "compute", "storage", "control-plane"},
			ProvidesCapabilities: []Capability{CapObjectStore},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-minio.service", Port: 9000},
		},
		{
			Name:                 "scylladb",
			Unit:                 "scylla-server.service",
			Kind:                 KindInfrastructure,
			Priority:             6,
			Profiles:             []string{"control-plane", "storage", "scylla", "database"},
			ProvidesCapabilities: []Capability{CapLocalDB},
			InstallMode:          InstallModeDay0Join, // OS package (apt install), not a repo artifact
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
			Kind:                 KindWorkload,
			Priority:             8,
			Profiles:             []string{"core", "compute", "control-plane"},
			ProvidesCapabilities: []Capability{CapMonitoring},
			ManagedUnit:          true,
			HealthCheck:          &HealthCheckHintC{Unit: "globular-monitoring.service"},
		},
		{
			Name:                 "xds",
			Unit:                 "globular-xds.service",
			Kind:                 KindInfrastructure,
			Priority:             9,
			Profiles:             []string{"control-plane", "gateway"},
			ProvidesCapabilities: []Capability{CapServiceMesh},
			HealthCheck:          &HealthCheckHintC{Unit: "globular-xds.service"},
		},
		{
			Name:                 "gateway",
			Unit:                 "globular-gateway.service",
			Kind:                 KindInfrastructure,
			Priority:             9,
			Profiles:             []string{"control-plane", "gateway"},
			ProvidesCapabilities: []Capability{CapGateway},
			RuntimeLocalDependencies: []string{"xds", "envoy"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-gateway.service", Port: 8080},
		},
		{
			Name:                 "envoy",
			Unit:                 "globular-envoy.service",
			Kind:                 KindInfrastructure,
			Priority:             10,
			Profiles:             []string{"control-plane", "gateway"},
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
			RuntimeLocalDependencies: []string{"event", "rclone", "restic", "sctool"},
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
			Profiles:                 []string{"core", "compute", "control-plane", "scylla", "database"},
			RuntimeLocalDependencies: []string{"scylladb", "event"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-ai-memory.service", Port: 10200},
		},
		{
			Name:                     "ai-executor",
			Unit:                     "globular-ai-executor.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute", "control-plane", "scylla", "database"},
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
			Profiles:                 []string{"core", "compute", "control-plane", "scylla", "database"},
			RuntimeLocalDependencies: []string{"ai-executor", "event"},
		},
		{
			Name:                     "workflow",
			Unit:                     "globular-workflow.service",
			Kind:                     KindWorkload,
			Priority:                 900, // before other AI services that may record to it
			Profiles:                 []string{"core", "compute", "control-plane", "scylla", "database"},
			RuntimeLocalDependencies: []string{"scylladb", "event"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-workflow.service", Port: 10220},
		},
		{
			Name:                     "mcp",
			Unit:                     "globular-mcp.service",
			Kind:                     KindWorkload,
			Priority:                 1000,
			Profiles:                 []string{"core", "compute"},
			RuntimeLocalDependencies: []string{"event"},
		},

		// ---------------------------------------------------------------
		// Command packages — CLI tools dispatched as dependencies
		// ---------------------------------------------------------------
		{
			Name:     "rclone",
			Kind:     KindCommand,
			Priority: 900,
			Profiles: []string{"core", "compute", "storage"},
		},
		{
			Name:     "restic",
			Kind:     KindCommand,
			Priority: 900,
			Profiles: []string{"core", "compute", "storage"},
		},
		{
			Name:                     "sctool",
			Kind:                     KindCommand,
			Priority:                 900,
			Profiles:                 []string{"core", "compute", "control-plane"},
			RuntimeLocalDependencies: []string{"scylla-manager"},
		},
		{
			Name:     "mc",
			Kind:     KindCommand,
			Priority: 900,
			Profiles: []string{"core", "compute", "storage"},
		},
		{
			Name:     "ffmpeg",
			Kind:     KindCommand,
			Priority: 900,
			Profiles: []string{"core", "compute"},
		},

		// ---------------------------------------------------------------
		// Infrastructure components — monitoring & database management
		// ---------------------------------------------------------------
		{
			Name:        "prometheus",
			Unit:        "globular-prometheus.service",
			Kind:        KindInfrastructure,
			Priority:    11,
			Profiles:    []string{"core", "compute", "control-plane"},
			HealthCheck: &HealthCheckHintC{Unit: "globular-prometheus.service", Port: 9090},
		},
		{
			Name:        "alertmanager",
			Unit:        "globular-alertmanager.service",
			Kind:        KindInfrastructure,
			Priority:    11,
			Profiles:    []string{"core", "compute", "control-plane"},
			HealthCheck: &HealthCheckHintC{Unit: "globular-alertmanager.service", Port: 9093},
		},
		{
			Name:        "node-exporter",
			Unit:        "globular-node-exporter.service",
			Kind:        KindInfrastructure,
			Priority:    11,
			Profiles:    []string{"core", "compute", "control-plane"},
			HealthCheck: &HealthCheckHintC{Unit: "globular-node-exporter.service", Port: 9100},
		},
		{
			Name:                     "scylla-manager",
			Unit:                     "globular-scylla-manager.service",
			Kind:                     KindInfrastructure,
			Priority:                 12,
			Profiles:                 []string{"core", "compute", "control-plane"},
			RuntimeLocalDependencies: []string{"scylladb"},
			InstallMode:              InstallModeDay0Join, // apt package, not a repo artifact
			HealthCheck:              &HealthCheckHintC{Unit: "globular-scylla-manager.service", Port: 5080},
		},
		{
			Name:                     "scylla-manager-agent",
			Unit:                     "globular-scylla-manager-agent.service",
			Kind:                     KindInfrastructure,
			Priority:                 12,
			Profiles:                 []string{"core", "compute", "control-plane"},
			RuntimeLocalDependencies: []string{"scylladb"},
			InstallMode:              InstallModeDay0Join, // apt package, not a repo artifact
			HealthCheck:              &HealthCheckHintC{Unit: "globular-scylla-manager-agent.service", Port: 10001},
		},
		{
			Name:                     "sidekick",
			Unit:                     "globular-sidekick.service",
			Kind:                     KindInfrastructure,
			Priority:                 11,
			Profiles:                 []string{"core", "compute", "storage"},
			RuntimeLocalDependencies: []string{"minio"},
			HealthCheck:              &HealthCheckHintC{Unit: "globular-sidekick.service"},
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

	// Derive profile vars from catalog for service_config.go renderers.
	deriveProfileVarsFromCatalog()
}

// deriveProfileVarsFromCatalog updates the profilesFor* vars in service_config.go
// from catalog entries. If a component is not in the catalog, the existing
// hardcoded defaults remain.
func deriveProfileVarsFromCatalog() {
	type profileBinding struct {
		name     string
		target   *[]string
	}
	bindings := []profileBinding{
		{"etcd", &profilesForEtcd},
		{"minio", &profilesForMinio},
		{"xds", &profilesForXDS},
		{"dns", &profilesForDNS},
		{"scylladb", &profilesForScyllaDB},
	}
	for _, b := range bindings {
		c := CatalogByName(b.name)
		if c != nil && len(c.Profiles) > 0 {
			*b.target = append([]string(nil), c.Profiles...)
		}
	}
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
		if c.Unit != "" {
			unitLower := strings.ToLower(c.Unit)
			if units[unitLower] {
				return fmt.Errorf("duplicate unit: %q", c.Unit)
			}
			units[unitLower] = true
		}
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

// ---------------------------------------------------------------------------
// Dynamic catalog loading from repository
// ---------------------------------------------------------------------------

// LoadCatalogFromRepository fetches all artifact manifests from the repository
// and builds a dynamic component catalog. Components missing from the repo
// fall back to static entries from buildCatalog().
func LoadCatalogFromRepository(repoAddr string) error {
	if repoAddr == "" {
		repoAddr = config.ResolveServiceAddr("repository.PackageRepository", "")
		if repoAddr == "" {
			return fmt.Errorf("cannot resolve repository service address from etcd")
		}
	}

	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository at %s: %w", repoAddr, err)
	}
	defer client.Close()

	artifacts, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	// Build dynamic catalog from repository artifacts.
	dynamic := make(map[string]*Component)
	for _, art := range artifacts {
		c := artifactToComponent(art)
		if c == nil {
			continue
		}
		// Deduplicate by name — latest version wins (ListArtifacts returns
		// sorted by version descending, so first occurrence is newest).
		if _, exists := dynamic[c.Name]; !exists {
			dynamic[c.Name] = c
		}
	}

	// Merge: dynamic entries take priority, static entries fill gaps.
	staticCatalog := buildCatalog()
	var merged []*Component
	seen := make(map[string]bool)

	// Add all dynamic entries first.
	for _, c := range dynamic {
		merged = append(merged, c)
		seen[c.Name] = true
	}

	// Fill in static entries not present in repository.
	for _, c := range staticCatalog {
		if !seen[c.Name] {
			merged = append(merged, c)
			seen[c.Name] = true
		}
	}

	// Sort by priority then name for deterministic ordering.
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Priority != merged[j].Priority {
			return merged[i].Priority < merged[j].Priority
		}
		return merged[i].Name < merged[j].Name
	})

	// Validate before swapping.
	oldCatalog := catalog
	oldIndex := catalogIndex
	oldByUnit := catalogByUnit
	catalog = merged
	catalogIndex = make(map[string]*Component, len(merged))
	catalogByUnit = make(map[string]*Component, len(merged))
	for _, c := range merged {
		catalogIndex[c.Name] = c
		catalogByUnit[strings.ToLower(c.Unit)] = c
	}

	if err := ValidateCatalog(); err != nil {
		// Roll back on validation failure.
		catalog = oldCatalog
		catalogIndex = oldIndex
		catalogByUnit = oldByUnit
		return fmt.Errorf("dynamic catalog validation failed: %w", err)
	}

	// Rebuild derived maps and profile capabilities.
	rebuildDerivedMaps()
	rebuildProfileCapabilities()

	log.Printf("loaded dynamic catalog from repository (%d components, %d from repo, %d static fallback)",
		len(merged), len(dynamic), len(merged)-len(dynamic))
	return nil
}

// artifactToComponent converts an ArtifactManifest to a Component.
// Returns nil if the manifest lacks the catalog metadata fields needed.
func artifactToComponent(art *repopb.ArtifactManifest) *Component {
	if art == nil || art.GetRef() == nil {
		return nil
	}
	ref := art.GetRef()
	name := ref.GetName()
	if name == "" {
		return nil
	}

	// Skip artifacts without catalog metadata (no profiles = not catalog-aware).
	profiles := art.GetProfiles()
	if len(profiles) == 0 {
		return nil
	}

	kind := KindWorkload
	switch ref.GetKind() {
	case repopb.ArtifactKind_INFRASTRUCTURE:
		kind = KindInfrastructure
	case repopb.ArtifactKind_COMMAND:
		kind = KindCommand
	}

	priority := int(art.GetPriority())
	if priority == 0 {
		priority = 1000 // default workload priority
	}

	systemdUnit := art.GetSystemdUnit()
	if systemdUnit == "" {
		systemdUnit = "globular-" + name + ".service"
	}

	var capabilities []Capability
	for _, cap := range art.GetProvides() {
		capabilities = append(capabilities, Capability(cap))
	}

	var healthCheck *HealthCheckHintC
	if art.GetHealthCheckUnit() != "" || art.GetHealthCheckPort() > 0 {
		healthCheck = &HealthCheckHintC{
			Unit: art.GetHealthCheckUnit(),
			Port: int(art.GetHealthCheckPort()),
		}
	}

	return &Component{
		Name:                     name,
		Unit:                     systemdUnit,
		Kind:                     kind,
		Priority:                 priority,
		Profiles:                 profiles,
		ProvidesCapabilities:     capabilities,
		InstallDependencies:      art.GetInstallDependencies(),
		RuntimeLocalDependencies: art.GetRuntimeLocalDependencies(),
		ManagedUnit:              art.GetManagedUnit(),
		InstallMode:              art.GetInstallMode(),
		HealthCheck:              healthCheck,
	}
}

// rebuildProfileCapabilities derives ProfileCapabilities from the catalog.
// For each profile, collects ProvidesCapabilities from all components listing that profile.
func rebuildProfileCapabilities() {
	derived := make(map[string][]Capability)
	for _, c := range catalog {
		for _, p := range c.Profiles {
			for _, cap := range c.ProvidesCapabilities {
				// Append unique.
				found := false
				for _, existing := range derived[p] {
					if existing == cap {
						found = true
						break
					}
				}
				if !found {
					derived[p] = append(derived[p], cap)
				}
			}
		}
	}
	ProfileCapabilities = derived
}

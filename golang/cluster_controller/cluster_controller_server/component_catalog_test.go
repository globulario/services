package main

import (
	"sort"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestCatalogIntegrity(t *testing.T) {
	if err := ValidateCatalog(); err != nil {
		t.Fatalf("catalog validation failed: %v", err)
	}
}

func TestCatalogNoDuplicateNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, c := range catalog {
		if seen[c.Name] {
			t.Errorf("duplicate component name: %q", c.Name)
		}
		seen[c.Name] = true
	}
}

func TestCatalogNoDuplicateUnits(t *testing.T) {
	seen := make(map[string]bool)
	for _, c := range catalog {
		u := strings.ToLower(c.Unit)
		if seen[u] {
			t.Errorf("duplicate unit: %q (component %q)", c.Unit, c.Name)
		}
		seen[u] = true
	}
}

func TestCatalogNoCycles(t *testing.T) {
	for _, c := range catalog {
		if err := checkCycle(c.Name, nil); err != nil {
			t.Errorf("cycle detected starting from %q: %v", c.Name, err)
		}
	}
}

func TestCatalogAllDepsExist(t *testing.T) {
	for _, c := range catalog {
		for _, dep := range c.InstallDependencies {
			if CatalogByName(dep) == nil {
				t.Errorf("component %q: install dep %q not in catalog", c.Name, dep)
			}
		}
		for _, dep := range c.RuntimeLocalDependencies {
			if CatalogByName(dep) == nil {
				t.Errorf("component %q: runtime dep %q not in catalog", c.Name, dep)
			}
		}
	}
}

func TestCatalogAllProfilesValid(t *testing.T) {
	for _, c := range catalog {
		for _, p := range c.Profiles {
			if err := ValidateProfile(p); err != nil {
				t.Errorf("component %q: %v", c.Name, err)
			}
		}
	}
}

// TestDerivedProfileUnitMap verifies the catalog-derived profileUnitMap
// contains all the units from the old hardcoded version, plus xds which
// was previously missing from profileUnitMap but present in service_config.go
// profilesForXDS. The catalog correctly includes xds in all profiles that
// need it.
func TestDerivedProfileUnitMap(t *testing.T) {
	// Verify all old units are present in the derived map.
	coreUnits := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-event.service",
		"globular-rbac.service",
		"globular-file.service",
		"globular-minio.service",
		"globular-monitoring.service",
	}
	mustContain := map[string][]string{
		"core":          coreUnits,
		"compute":       coreUnits,
		"control-plane": {"globular-etcd.service", "globular-dns.service", "globular-discovery.service"},
		"gateway":       {"globular-gateway.service", "globular-envoy.service"},
		"storage":       {"globular-minio.service", "globular-file.service"},
		"dns":           {"globular-dns.service"},
		"scylla":        {"scylla-server.service"},
		"database":      {"scylla-server.service"},
	}

	for profile, required := range mustContain {
		derivedUnits, ok := profileUnitMap[profile]
		if !ok {
			t.Errorf("derived map missing profile %q", profile)
			continue
		}
		derivedSet := make(map[string]bool)
		for _, u := range derivedUnits {
			derivedSet[u] = true
		}
		for _, unit := range required {
			if !derivedSet[unit] {
				t.Errorf("profile %q: missing required unit %q (got %v)", profile, unit, derivedUnits)
			}
		}
	}
}

// TestDerivedUnitTier verifies the catalog-derived unitTier matches old hardcoded version.
func TestDerivedUnitTier(t *testing.T) {
	oldTier := map[string]ServiceTier{
		"globular-etcd.service":    TierInfrastructure,
		"globular-xds.service":     TierInfrastructure,
		"globular-envoy.service":   TierInfrastructure,
		"globular-minio.service":   TierInfrastructure,
		"globular-gateway.service": TierInfrastructure,
		"scylla-server.service":    TierInfrastructure,
	}

	for unit, oldT := range oldTier {
		derivedT := getUnitTier(unit)
		if derivedT != oldT {
			t.Errorf("unit %q: old tier=%d derived tier=%d", unit, oldT, derivedT)
		}
	}
}

// TestDerivedUnitPriority verifies the catalog-derived unitPriority matches old hardcoded version.
func TestDerivedUnitPriority(t *testing.T) {
	oldPriority := map[string]int{
		"globular-etcd.service":       1,
		"globular-dns.service":        2,
		"globular-discovery.service":  3,
		"globular-event.service":      4,
		"globular-rbac.service":       5,
		"globular-minio.service":      6,
		"globular-file.service":       7,
		"globular-monitoring.service": 8,
		"globular-gateway.service":    9,
		"globular-xds.service":        9,
		"globular-envoy.service":      10,
		"scylla-server.service":       6,
	}

	for unit, oldP := range oldPriority {
		derivedP := getUnitPriority(unit)
		if derivedP != oldP {
			t.Errorf("unit %q: old priority=%d derived priority=%d", unit, oldP, derivedP)
		}
	}
}

func TestCatalogByName(t *testing.T) {
	c := CatalogByName("etcd")
	if c == nil {
		t.Fatal("etcd not found")
	}
	if c.Unit != "globular-etcd.service" {
		t.Errorf("etcd unit: got %q want %q", c.Unit, "globular-etcd.service")
	}
	if c.Kind != KindInfrastructure {
		t.Error("etcd should be KindInfrastructure")
	}
}

func TestCatalogByUnitName(t *testing.T) {
	c := CatalogByUnitName("scylla-server.service")
	if c == nil {
		t.Fatal("scylla-server.service not found")
	}
	if c.Name != "scylladb" {
		t.Errorf("got name %q want %q", c.Name, "scylladb")
	}
}

func TestComponentsForProfile(t *testing.T) {
	comps := ComponentsForProfile("database")
	names := make([]string, len(comps))
	for i, c := range comps {
		names[i] = c.Name
	}
	// database profile should include scylladb and AI workloads
	want := map[string]bool{
		"scylladb":    true,
		"ai-memory":   true,
		"ai-executor": true,
		"ai-watcher":  true,
	}
	for _, n := range names {
		delete(want, n)
	}
	for missing := range want {
		t.Errorf("database profile missing component %q", missing)
	}
}

func TestComponentsProvidingCapability(t *testing.T) {
	comps := ComponentsProvidingCapability(CapLocalDB)
	if len(comps) != 1 {
		t.Fatalf("expected 1 component providing local-db, got %d", len(comps))
	}
	if comps[0].Name != "scylladb" {
		t.Errorf("expected scylladb, got %q", comps[0].Name)
	}
}

func TestProfilesForComponent(t *testing.T) {
	profiles := ProfilesForComponent("etcd")
	sort.Strings(profiles)
	want := []string{"compute", "control-plane", "core"}
	if len(profiles) != len(want) {
		t.Fatalf("etcd profiles: got %v want %v", profiles, want)
	}
	for i := range want {
		if profiles[i] != want[i] {
			t.Errorf("etcd profile[%d]: got %q want %q", i, profiles[i], want[i])
		}
	}
}

// TestXdsInGatewayProfile verifies xds is included for gateway profile.
// This was previously handled by the hardcoded profileUnitMap having xds in
// core/compute/control-plane/gateway.
func TestXdsInGatewayProfile(t *testing.T) {
	units, ok := profileUnitMap["gateway"]
	if !ok {
		t.Fatal("gateway profile missing from profileUnitMap")
	}
	found := false
	for _, u := range units {
		if u == "globular-xds.service" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("gateway profile should include xds unit, got: %v", units)
	}
}

// TestArtifactToComponent verifies conversion from ArtifactManifest to Component.
func TestArtifactToComponent(t *testing.T) {
	art := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			Name: "test-service",
			Kind: repopb.ArtifactKind_SERVICE,
		},
		Profiles:                 []string{"core", "compute"},
		Priority:                 42,
		InstallMode:              "repository",
		ManagedUnit:              true,
		SystemdUnit:              "globular-test-service.service",
		Provides:                 []string{"test-cap"},
		InstallDependencies:      []string{},
		RuntimeLocalDependencies: []string{"event"},
		HealthCheckUnit:          "globular-test-service.service",
		HealthCheckPort:          9999,
	}

	c := artifactToComponent(art)
	if c == nil {
		t.Fatal("expected non-nil component")
	}
	if c.Name != "test-service" {
		t.Errorf("name: got %q want %q", c.Name, "test-service")
	}
	if c.Kind != KindWorkload {
		t.Error("SERVICE artifact should map to KindWorkload")
	}
	if c.Priority != 42 {
		t.Errorf("priority: got %d want %d", c.Priority, 42)
	}
	if !c.ManagedUnit {
		t.Error("managed_unit should be true")
	}
	if c.Unit != "globular-test-service.service" {
		t.Errorf("unit: got %q want %q", c.Unit, "globular-test-service.service")
	}
	if len(c.ProvidesCapabilities) != 1 || c.ProvidesCapabilities[0] != "test-cap" {
		t.Errorf("provides: got %v want [test-cap]", c.ProvidesCapabilities)
	}
	if len(c.RuntimeLocalDependencies) != 1 || c.RuntimeLocalDependencies[0] != "event" {
		t.Errorf("runtime deps: got %v want [event]", c.RuntimeLocalDependencies)
	}
	if c.HealthCheck == nil {
		t.Fatal("expected health check")
	}
	if c.HealthCheck.Port != 9999 {
		t.Errorf("health port: got %d want %d", c.HealthCheck.Port, 9999)
	}
}

// TestArtifactToComponent_Infrastructure verifies INFRASTRUCTURE kind mapping.
func TestArtifactToComponent_Infrastructure(t *testing.T) {
	art := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			Name: "my-infra",
			Kind: repopb.ArtifactKind_INFRASTRUCTURE,
		},
		Profiles: []string{"core"},
		Priority: 5,
	}
	c := artifactToComponent(art)
	if c == nil {
		t.Fatal("expected non-nil")
	}
	if c.Kind != KindInfrastructure {
		t.Error("INFRASTRUCTURE artifact should map to KindInfrastructure")
	}
	if c.Unit != "globular-my-infra.service" {
		t.Errorf("auto-derived unit: got %q want %q", c.Unit, "globular-my-infra.service")
	}
}

// TestArtifactToComponent_NilAndNoProfiles verifies nil/empty cases return nil.
func TestArtifactToComponent_NilAndNoProfiles(t *testing.T) {
	if artifactToComponent(nil) != nil {
		t.Error("nil manifest should return nil")
	}
	// Manifest without profiles is not catalog-aware.
	art := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{Name: "foo", Kind: repopb.ArtifactKind_SERVICE},
	}
	if artifactToComponent(art) != nil {
		t.Error("manifest without profiles should return nil")
	}
}

// TestDerivedProfileCapabilities verifies that ProfileCapabilities is correctly
// derived from the catalog components' ProvidesCapabilities fields.
func TestDerivedProfileCapabilities(t *testing.T) {
	// Rebuild to ensure current state.
	rebuildProfileCapabilities()

	// "core" profile should include config-store (from etcd), dns, service-discovery,
	// event-bus, object-store, monitoring, local-db (from scylladb)
	coreCaps := ProfileCapabilities["core"]
	if coreCaps == nil {
		t.Fatal("core profile missing from ProfileCapabilities")
	}
	required := []Capability{CapConfigStore, CapDNS, CapServiceDiscovery, CapEventBus, CapObjectStore, CapLocalDB}
	capSet := make(map[Capability]bool)
	for _, c := range coreCaps {
		capSet[c] = true
	}
	for _, r := range required {
		if !capSet[r] {
			t.Errorf("core profile missing capability %q (got %v)", r, coreCaps)
		}
	}

	// "gateway" profile should have http-proxy, service-mesh, gateway.
	gwCaps := ProfileCapabilities["gateway"]
	if gwCaps == nil {
		t.Fatal("gateway profile missing from ProfileCapabilities")
	}
	gwSet := make(map[Capability]bool)
	for _, c := range gwCaps {
		gwSet[c] = true
	}
	for _, r := range []Capability{CapHTTPProxy, CapServiceMesh, CapGateway} {
		if !gwSet[r] {
			t.Errorf("gateway profile missing capability %q (got %v)", r, gwCaps)
		}
	}
}

// TestNodeHasComponentProfile verifies catalog-driven profile checks.
func TestNodeHasComponentProfile(t *testing.T) {
	coreNode := &nodeState{Profiles: []string{"core", "compute"}}
	gwNode := &nodeState{Profiles: []string{"gateway"}}

	if !nodeHasEtcdProfile(coreNode) {
		t.Error("core node should have etcd profile")
	}
	if nodeHasEtcdProfile(gwNode) {
		t.Error("gateway node should NOT have etcd profile")
	}
	if !nodeHasEnvoyProfile(gwNode) {
		t.Error("gateway node should have envoy profile")
	}
	if nodeHasEnvoyProfile(coreNode) {
		t.Error("core node should NOT have envoy profile")
	}
}

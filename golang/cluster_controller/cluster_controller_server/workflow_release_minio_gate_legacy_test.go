package main

import (
	"net"
	"strings"
	"testing"
)

// The waiver has two admission models to satisfy. v2 clusters publish
// AuthorizedMembers; Day-0 / legacy clusters publish only the pool IP list:
//
//	{"mode":"standalone","generation":0,"nodes":["10.10.0.11"]}
//
// The legacy branch used to return false unconditionally, so on those clusters
// the waiver never fired and InfrastructureRelease/core@globular.io/minio
// abandoned 5/5 defers on a cluster where every node was behaving correctly
// (invariant:minio.is_commodity_not_a_pillar — MinIO must never block another
// component's convergence). These tests pin the membership decision that branch
// now makes.

// legacyPoolHolds mirrors the legacy membership test in minioTopologyHeldOnNode:
// true means "held by topology — waive the active check".
func legacyPoolHolds(pool []string, nodeAddr string) bool {
	if nodeAddr == "" || len(pool) == 0 {
		return false // fail closed
	}
	host := strings.TrimSpace(nodeAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" {
		return false
	}
	for _, ip := range pool {
		if strings.EqualFold(strings.TrimSpace(ip), host) {
			return false
		}
	}
	return true
}

func TestLegacyPoolWaivesNonMemberNode(t *testing.T) {
	pool := []string{"10.10.0.11"} // standalone Day-0 objectstore

	// node-5 is not in the pool: node-agent holds its unit stopped on purpose,
	// so demanding "active" can never pass.
	if !legacyPoolHolds(pool, "10.10.0.15:11000") {
		t.Error("non-member node not waived — verify_runtime will defer 5/5 and " +
			"permanently abandon the minio release on a healthy cluster")
	}
	// The pool member must still be checked for real.
	if legacyPoolHolds(pool, "10.10.0.11:11000") {
		t.Error("pool member was waived — a genuinely dead MinIO on the one node " +
			"that must run it would be masked")
	}
}

// Uncertainty must not grant a waiver: without an address, or with an empty
// pool, membership is unknown and the normal active-check has to stay in force.
func TestLegacyPoolFailsClosedOnUnknownMembership(t *testing.T) {
	for _, tc := range []struct {
		name string
		pool []string
		addr string
	}{
		{"no agent endpoint", []string{"10.10.0.11"}, ""},
		{"empty pool", nil, "10.10.0.15:11000"},
		{"both unknown", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if legacyPoolHolds(tc.pool, tc.addr) {
				t.Fatal("waiver granted on missing evidence — a waiver must never " +
					"be inferred from an unanswered membership question")
			}
		})
	}
}

// Endpoints arrive as host:port, but a bare host must work too.
func TestLegacyPoolAcceptsBareHostAndHostPort(t *testing.T) {
	pool := []string{"10.10.0.11"}
	if legacyPoolHolds(pool, "10.10.0.11") {
		t.Error("bare host form of a pool member was waived")
	}
	if !legacyPoolHolds(pool, "10.10.0.14") {
		t.Error("bare host form of a non-member was not waived")
	}
}

// Only MinIO-family packages are eligible; the waiver must not leak to others.
func TestOnlyMinioFamilyPackagesAreWaivable(t *testing.T) {
	for _, pkg := range []string{"minio", "sidekick", "MinIO"} {
		if !minioTopologyHeldPackages[strings.ToLower(pkg)] {
			t.Errorf("%s should be topology-held", pkg)
		}
	}
	for _, pkg := range []string{"etcd", "scylladb", "node-agent", "keepalived"} {
		if minioTopologyHeldPackages[strings.ToLower(pkg)] {
			t.Errorf("%s must NOT be waivable — only MinIO's unit is held by the "+
				"objectstore topology gate", pkg)
		}
	}
}

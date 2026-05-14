package main

// dns_readiness.go — Translate per-node heartbeat data (installed packages,
// systemd unit states, lifecycle flags) into the readiness inputs the DNS
// reconciler needs for 4-layer gating.
//
// Background: prior to this file the DNS reconciler published records purely
// from cluster membership + Profile. That violated the 4-layer model — a node
// whose `gateway` package was still PLANNED (Layer 2 only) appeared in
// `gateway.<domain>` records, causing clients to dial half-dead boxes. The
// reconciler now requires that a record's referenced service be both Installed
// (Layer 3) AND Runtime-healthy (Layer 4) on each candidate node; these
// helpers extract those sets from nodeState.
//
// One design choice: when a node has reported neither InstalledVersions nor
// any unit statuses (cold start, just joined, agent crashed), we return nil
// for the corresponding map. NodeInfo.ServiceReady falls back to profile-only
// in that case so the bootstrap path can still publish controller / dns
// records before any heartbeat round-trip completes. As soon as the first
// heartbeat arrives with real data, the gate engages.

import (
	"strings"
)

// dnsServiceUnitName maps a DNS-record-relevant service identifier (the
// package name as stored in InstalledVersions / used in NodeInfo.ServiceReady)
// to the systemd unit whose state determines runtime health.
//
// Most globular services follow `globular-<name>.service`; a few well-known
// services do not (scylladb runs under `scylla-server.service`, etcd under
// `globular-etcd.service` which already matches the pattern, etc.). Keeping
// this mapping local and explicit avoids guessing — a wrong guess would cause
// silent DNS withdrawal.
var dnsServiceUnitName = map[string]string{
	"gateway":            "globular-gateway.service",
	"dns":                "globular-dns.service",
	"cluster-controller": "globular-cluster-controller.service",
	"scylladb":           "scylla-server.service",
	"etcd":               "globular-etcd.service",
	"envoy":              "globular-envoy.service",
	"xds":                "globular-xds.service",
	"rbac":               "globular-rbac.service",
	"mcp":                "globular-mcp.service",
	"repository":         "globular-repository.service",
	"workflow":           "globular-workflow.service",
	"authentication":     "globular-authentication.service",
	"resource":           "globular-resource.service",
	"event":              "globular-event.service",
}

// buildInstalledServiceSet returns a set of package names that the node-agent
// has confirmed installed. nil ⇒ no installed data reported yet (cold start).
func buildInstalledServiceSet(node *nodeState) map[string]bool {
	if node == nil || node.InstalledVersions == nil {
		return nil
	}
	set := make(map[string]bool, len(node.InstalledVersions))
	for pkg, version := range node.InstalledVersions {
		if strings.TrimSpace(pkg) == "" || strings.TrimSpace(version) == "" {
			continue
		}
		set[pkg] = true
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// buildRuntimeHealthySet returns the set of services whose systemd unit is in
// "active" state on this node. nil ⇒ no unit data reported yet.
//
// A service is healthy iff its mapped unit is present in the node's reported
// units AND its State is "active". Missing / inactive / failed all map to
// "not healthy", and the DNS reconciler will refuse to publish a record that
// would resolve to this node for that service.
func buildRuntimeHealthySet(node *nodeState) map[string]bool {
	if node == nil || len(node.Units) == 0 {
		return nil
	}
	unitState := make(map[string]string, len(node.Units))
	for _, u := range node.Units {
		unitState[strings.TrimSpace(u.Name)] = strings.ToLower(strings.TrimSpace(u.State))
	}
	healthy := make(map[string]bool, len(dnsServiceUnitName))
	for svc, unit := range dnsServiceUnitName {
		if state, ok := unitState[unit]; ok && state == "active" {
			healthy[svc] = true
		}
	}
	if len(healthy) == 0 {
		// Distinguish "we received unit data, but nothing matched" from "no
		// data at all". Return a non-nil empty map so the gate fails closed —
		// the node WILL be filtered from every gated record. This is the
		// safe behavior: a node that reports its units and has none active
		// is not eligible for any service-bearing DNS record.
		return map[string]bool{}
	}
	return healthy
}

// isNodeDraining reports whether the node is being intentionally removed from
// service. A draining node must withdraw from DNS at the start of removal,
// not after services stop, so clients stop receiving its address before its
// listeners go silent.
func isNodeDraining(node *nodeState) bool {
	if node == nil {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(node.Status))
	switch status {
	case "draining", "removing", "removed", "decommissioning":
		return true
	}
	return false
}

// isNodeQuarantined reports whether the node has been quarantined (a stronger
// state than draining — operator-asserted "do not route here"). Currently
// surfaced via BlockedReason; the DNS gate treats any blocked node as
// ineligible regardless of underlying reason.
func isNodeQuarantined(node *nodeState) bool {
	if node == nil {
		return false
	}
	return strings.TrimSpace(node.BlockedReason) != ""
}

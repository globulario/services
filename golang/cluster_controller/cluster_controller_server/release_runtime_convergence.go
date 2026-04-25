package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

const (
	// During bootstrap/rejoin we require fresher runtime observations.
	runtimeFreshnessBootstrap = 60 * time.Second
	// During steady-state we tolerate older heartbeats.
	runtimeFreshnessSteady = 3 * time.Minute
	// Per node/package/version runtime-repair cooldown.
	runtimeRepairCooldown = 45 * time.Second
)

type RuntimeConvergence string

const (
	RuntimeConverged RuntimeConvergence = "converged"
	RuntimeInactive  RuntimeConvergence = "inactive"
	RuntimeFailed    RuntimeConvergence = "failed"
	RuntimeMissing   RuntimeConvergence = "missing"
	RuntimeUnknown   RuntimeConvergence = "unknown"
	RuntimeStale     RuntimeConvergence = "stale"
	RuntimeNotNeeded RuntimeConvergence = "not_needed"
)

type PackageConvergence struct {
	VersionOK      bool
	HashOK         bool
	BuildIDOK      bool
	RuntimeNeeded  bool
	RuntimeOK      bool
	RuntimeState   RuntimeConvergence
	RepairRequired bool
	Reason         string
}

var runtimeRepairCooldownByTarget sync.Map // key -> time.Time

func runtimeProofRequiredForKind(pkgKind, pkgName string) bool {
	kind := strings.ToUpper(strings.TrimSpace(pkgKind))
	if kind == "COMMAND" {
		return false
	}
	if skipRuntimeCheck(pkgName) {
		return false
	}
	return kind == "SERVICE" || kind == "INFRASTRUCTURE" || kind == "APPLICATION"
}

func runtimeUnitForPackage(pkgName, pkgKind string) string {
	name := strings.TrimSpace(pkgName)
	kind := strings.ToUpper(strings.TrimSpace(pkgKind))
	if kind == "SERVICE" || kind == "APPLICATION" {
		name = canonicalServiceName(name)
	}
	if name == "" {
		return ""
	}
	return packageToUnit(name)
}

func runtimeFreshnessThreshold(node *nodeState) time.Duration {
	if node == nil {
		return runtimeFreshnessSteady
	}
	switch node.BootstrapPhase {
	case BootstrapAdmitted, BootstrapInfraPreparing, BootstrapEtcdJoining, BootstrapEtcdReady, BootstrapXdsReady, BootstrapEnvoyReady, BootstrapStorageJoining:
		return runtimeFreshnessBootstrap
	default:
		return runtimeFreshnessSteady
	}
}

func runtimeStatusFresh(node *nodeState, now time.Time) (bool, string) {
	if node == nil {
		return false, "runtime status unknown (node state unavailable)"
	}
	if node.LastSeen.IsZero() {
		return false, "runtime status unknown (no heartbeat)"
	}
	age := now.Sub(node.LastSeen)
	thr := runtimeFreshnessThreshold(node)
	if age > thr {
		return false, fmt.Sprintf("runtime status stale (%s > %s)", age.Round(time.Second), thr)
	}
	return true, ""
}

func classifyPackageConvergence(
	node *nodeState,
	pkgName, pkgKind string,
	desiredVersion, desiredHash, desiredBuildID string,
	installed *node_agentpb.InstalledPackage,
	now time.Time,
) PackageConvergence {
	pc := PackageConvergence{
		RuntimeNeeded: runtimeProofRequiredForKind(pkgKind, pkgName),
		RuntimeState:  RuntimeUnknown,
	}

	if installed == nil {
		pc.RepairRequired = true
		pc.Reason = "not installed"
		if !pc.RuntimeNeeded {
			pc.RuntimeState = RuntimeNotNeeded
		}
		return pc
	}

	gotVersion := strings.TrimSpace(installed.GetVersion())
	wantVersion := strings.TrimSpace(desiredVersion)
	if wantVersion == "" || gotVersion == wantVersion {
		pc.VersionOK = true
	} else {
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("installed version %s != desired %s", gotVersion, wantVersion)
		return pc
	}

	gotHash := normalizeDesiredHash(installed.GetChecksum())
	wantHash := normalizeDesiredHash(desiredHash)
	if wantHash == "" || gotHash == wantHash {
		pc.HashOK = true
	} else {
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("installed checksum %s != desired %s", gotHash, wantHash)
		return pc
	}

	gotBuild := strings.TrimSpace(installed.GetBuildId())
	wantBuild := strings.TrimSpace(desiredBuildID)
	if wantBuild == "" || gotBuild == wantBuild {
		pc.BuildIDOK = true
	} else {
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("installed build_id %s != desired %s", gotBuild, wantBuild)
		return pc
	}

	if !pc.RuntimeNeeded {
		pc.RuntimeOK = true
		pc.RuntimeState = RuntimeNotNeeded
		pc.Reason = "runtime not needed"
		return pc
	}

	fresh, freshnessReason := runtimeStatusFresh(node, now)
	if !fresh {
		pc.RepairRequired = true
		pc.RuntimeState = RuntimeStale
		pc.Reason = freshnessReason
		return pc
	}

	unit := runtimeUnitForPackage(pkgName, pkgKind)
	if unit == "" {
		pc.RepairRequired = true
		pc.RuntimeState = RuntimeUnknown
		pc.Reason = "runtime unit unknown"
		return pc
	}

	for _, u := range node.Units {
		if !strings.EqualFold(strings.TrimSpace(u.Name), unit) {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(u.State))
		switch state {
		case "active":
			pc.RuntimeOK = true
			pc.RuntimeState = RuntimeConverged
			pc.Reason = fmt.Sprintf("%s active", unit)
			return pc
		case "failed":
			pc.RuntimeState = RuntimeFailed
		case "inactive":
			pc.RuntimeState = RuntimeInactive
		case "":
			pc.RuntimeState = RuntimeUnknown
		default:
			pc.RuntimeState = RuntimeUnknown
		}
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("%s state=%s", unit, state)
		return pc
	}

	pc.RepairRequired = true
	pc.RuntimeState = RuntimeMissing
	pc.Reason = fmt.Sprintf("%s missing", unit)
	return pc
}

func packageRuntimeHealthyOnNode(node *nodeState, pkgName, pkgKind string) (bool, string) {
	// Build an artificial installed record so classifyPackageConvergence performs
	// runtime-only checks without version/hash/build gates.
	pc := classifyPackageConvergence(
		node,
		pkgName,
		pkgKind,
		"",
		"",
		"",
		&node_agentpb.InstalledPackage{Version: "runtime-check"},
		time.Now(),
	)
	return pc.RuntimeOK, pc.Reason
}

func runtimeRepairCooldownKey(nodeID, pkgName, pkgKind, desiredVersion, desiredHash, desiredBuildID string) string {
	return strings.ToLower(strings.TrimSpace(nodeID) + "|" + strings.TrimSpace(pkgKind) + "|" + strings.TrimSpace(pkgName) +
		"|" + strings.TrimSpace(desiredVersion) + "|" + normalizeDesiredHash(desiredHash) + "|" + strings.TrimSpace(desiredBuildID))
}

func shouldDispatchRuntimeRepair(key string, now time.Time) (bool, time.Duration) {
	if v, ok := runtimeRepairCooldownByTarget.Load(key); ok {
		last := v.(time.Time)
		if elapsed := now.Sub(last); elapsed < runtimeRepairCooldown {
			return false, runtimeRepairCooldown - elapsed
		}
	}
	runtimeRepairCooldownByTarget.Store(key, now)
	return true, 0
}

func normalizeDesiredHash(hash string) string {
	h := strings.ToLower(strings.TrimSpace(hash))
	h = strings.TrimPrefix(h, "sha256:")
	return h
}

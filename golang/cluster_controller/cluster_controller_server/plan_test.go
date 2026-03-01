package main

import (
	"strings"
	"testing"
)

func TestBuildPlanActionsStopDisableForRemovedUnits(t *testing.T) {
	// Transition from ["core","gateway"] → ["gateway"]
	// core units should get stop+disable, gateway units should get enable+start
	gatewayActions, err := buildPlanActions([]string{"gateway"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actionMap := make(map[string][]string) // unit → actions in order
	for _, a := range gatewayActions {
		actionMap[a.UnitName] = append(actionMap[a.UnitName], a.Action)
	}

	// Core-only units must have stop+disable
	coreOnlyUnits := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-event.service",
		"globular-rbac.service",
	}
	for _, unit := range coreOnlyUnits {
		actions := actionMap[unit]
		if len(actions) != 2 || actions[0] != "stop" || actions[1] != "disable" {
			t.Errorf("unit %s: expected [stop, disable], got %v", unit, actions)
		}
	}

	// Gateway units must have enable+start
	for _, unit := range []string{"globular-gateway.service", "envoy.service"} {
		actions := actionMap[unit]
		if len(actions) != 2 || actions[0] != "enable" || actions[1] != "start" {
			t.Errorf("unit %s: expected [enable, start], got %v", unit, actions)
		}
	}
}

func TestBuildPlanActionsStopDisableOrderBeforeEnableStart(t *testing.T) {
	// All stop/disable actions must appear before all enable/start actions.
	actions, err := buildPlanActions([]string{"gateway"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seenEnableOrStart := false
	for _, a := range actions {
		if a.Action == "enable" || a.Action == "start" {
			seenEnableOrStart = true
		}
		if seenEnableOrStart && (a.Action == "stop" || a.Action == "disable") {
			t.Errorf("stop/disable action for %s appears after enable/start", a.UnitName)
		}
	}
}

func TestBuildPlanActionsNoRemovedUnitsForCoreGateway(t *testing.T) {
	// core+gateway covers all units in allManagedUnits (core units + gateway units).
	// There should be no stop/disable actions because nothing is removed.
	coreGatewayUnits := make(map[string]struct{})
	for _, u := range profileUnitMap["core"] {
		coreGatewayUnits[strings.ToLower(u)] = struct{}{}
	}
	for _, u := range profileUnitMap["gateway"] {
		coreGatewayUnits[strings.ToLower(u)] = struct{}{}
	}

	// Only test if all allManagedUnits are in the combined set.
	allCovered := true
	for _, u := range allManagedUnits {
		if _, ok := coreGatewayUnits[u]; !ok {
			allCovered = false
			break
		}
	}

	if !allCovered {
		// Some profiles were added that aren't covered — skip this assertion.
		t.Skip("not all managed units covered by core+gateway, test needs updating")
		return
	}

	actions, err := buildPlanActions([]string{"core", "gateway"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Action == "stop" || a.Action == "disable" {
			t.Errorf("unexpected stop/disable action for %s when all managed units are desired", a.UnitName)
		}
	}
}

func TestBuildPlanActionsOverlapIdempotent(t *testing.T) {
	// A unit in both a removed profile and a desired profile should only get
	// enable+start (desired wins, it should not appear in removed).
	// minio.service is in both "core" and "storage".
	// If profiles = ["storage"], minio is desired, so it should NOT get stop.
	actions, err := buildPlanActions([]string{"storage"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.UnitName == "globular-minio.service" && (a.Action == "stop" || a.Action == "disable") {
			t.Errorf("globular-minio.service should not get stop/disable when it is in desired profile")
		}
	}
}

func TestBuildPlanActionsUnknownProfile(t *testing.T) {
	_, err := buildPlanActions([]string{"nonexistent-profile"})
	if err == nil {
		t.Error("expected error for unknown profile")
	}
}

func TestBuildPlanActionsMultipleUnknownProfiles(t *testing.T) {
	_, err := buildPlanActions([]string{"bad1", "bad2"})
	if err == nil {
		t.Error("expected error for multiple unknown profiles")
	}
}

func TestBuildPlanActionsEmptyDefaultsToCore(t *testing.T) {
	actions, err := buildPlanActions(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected actions for default core profile")
	}
}

func TestAllManagedUnitsNotEmpty(t *testing.T) {
	if len(allManagedUnits) == 0 {
		t.Error("allManagedUnits should not be empty")
	}
}

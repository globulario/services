package main

import (
	"reflect"
	"testing"
)

func TestInstalledStateKindsToDeleteForUninstall_CommandDeletesSiblingKinds(t *testing.T) {
	got := installedStateKindsToDeleteForUninstall("command")
	want := []string{"COMMAND", "SERVICE", "INFRASTRUCTURE"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("COMMAND uninstall must clear stale sibling kinds before sync backfill: got %v want %v", got, want)
	}
}

func TestInstalledStateKindsToDeleteForUninstall_ServiceIsExactKind(t *testing.T) {
	got := installedStateKindsToDeleteForUninstall("SERVICE")
	want := []string{"SERVICE"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SERVICE uninstall must not clear sibling kinds: got %v want %v", got, want)
	}
}

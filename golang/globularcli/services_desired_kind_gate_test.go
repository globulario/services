package main

// services_desired_kind_gate_test.go — D2: the CLI half of invariant
// desired.keyed_by_kind_and_name. `services desired set` is SERVICE-only and
// fails closed; INFRASTRUCTURE/COMMAND are refused (the xds incident was
// `... set xds --force`, and --force is now gone).

import (
	"errors"
	"strings"
	"testing"

	"github.com/globulario/services/golang/repository/repositorypb"
)

func TestServiceDesiredKindGate_ServiceProceeds(t *testing.T) {
	if err := serviceDesiredKindGate("echo", repositorypb.ArtifactKind_SERVICE, nil); err != nil {
		t.Fatalf("verified SERVICE must proceed; got %v", err)
	}
}

// The xds incident: an INFRASTRUCTURE name through `services desired set` must be
// refused — no --force escape hatch exists anymore.
func TestServiceDesiredKindGate_InfrastructureRefused(t *testing.T) {
	err := serviceDesiredKindGate("xds", repositorypb.ArtifactKind_INFRASTRUCTURE, nil)
	if err == nil {
		t.Fatal("INFRASTRUCTURE name must be refused by `services desired set`")
	}
	if !strings.Contains(err.Error(), "INFRASTRUCTURE") {
		t.Fatalf("refusal should explain it is an INFRASTRUCTURE package; got %v", err)
	}
}

func TestServiceDesiredKindGate_CommandRefused(t *testing.T) {
	err := serviceDesiredKindGate("yt-dlp", repositorypb.ArtifactKind_COMMAND, nil)
	if err == nil || !strings.Contains(err.Error(), "COMMAND") {
		t.Fatalf("COMMAND name must be refused with a COMMAND message; got %v", err)
	}
}

// Fail-closed: a reachable-but-unknown kind (no published version) is refused,
// not silently passed through.
func TestServiceDesiredKindGate_UnknownKindFailsClosed(t *testing.T) {
	err := serviceDesiredKindGate("newsvc", repositorypb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED, nil)
	if err == nil {
		t.Fatal("unknown kind must fail closed (refuse), not proceed")
	}
	if !strings.Contains(err.Error(), "fails closed") {
		t.Fatalf("message should state it fails closed; got %v", err)
	}
}

// Fail-closed: repository unreachable (lookup error) is refused, with a message
// that names the repo-unreachable cause.
func TestServiceDesiredKindGate_RepoUnreachableFailsClosed(t *testing.T) {
	err := serviceDesiredKindGate("echo", repositorypb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED, errors.New("dial tcp: connection refused"))
	if err == nil {
		t.Fatal("repository-unreachable lookup must fail closed (refuse)")
	}
	if !strings.Contains(err.Error(), "repository unreachable") {
		t.Fatalf("message should name the repo-unreachable cause; got %v", err)
	}
}

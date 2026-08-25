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

// TestKindGateUnspecifiedNamesTheInstanceAndBothCauses pins the attribution of
// an UNSPECIFIED kind.
//
// The message used to say only "publish the package first", which is the wrong
// advice whenever the package IS published. Kind resolution goes to whichever
// repository instance discovery selects, and an instance on a freshly-joined
// node registers before it has synced its artifact index. Observed 2026-08-23:
// three `desired set` calls were refused for dns, ai-watcher and authentication
// while `pkg info dns` reported kind SERVICE, publisher core@globular.io,
// version 1.2.317, installed on all five nodes. Six consecutive lookups
// auto-discovered six different endpoints and every one resolved SERVICE once
// the cluster settled.
//
// The gate must still fail closed — only the attribution changes.
func TestKindGateUnspecifiedNamesTheInstanceAndBothCauses(t *testing.T) {
	err := serviceDesiredKindGateAt("dns",
		repositorypb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED, nil, "10.10.0.15:443")
	if err == nil {
		t.Fatal("an unresolvable kind must still be refused — the gate fails closed")
	}
	msg := err.Error()

	if !strings.Contains(msg, "10.10.0.15:443") {
		t.Errorf("must name the instance that answered so the operator can check it; got: %s", msg)
	}
	if !strings.Contains(msg, "syncing") {
		t.Errorf("must offer the not-yet-synced cause, not only 'unpublished'; got: %s", msg)
	}
	if !strings.Contains(msg, "pkg info dns") {
		t.Errorf("must tell the operator how to distinguish the two causes; got: %s", msg)
	}
}

// An empty endpoint must degrade to prose, never to a dangling "instance ".
func TestKindGateUnspecifiedWithoutEndpointStaysReadable(t *testing.T) {
	err := serviceDesiredKindGateAt("dns",
		repositorypb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED, nil, "")
	if err == nil {
		t.Fatal("must still refuse")
	}
	if strings.Contains(err.Error(), "instance :") ||
		strings.Contains(err.Error(), "instance  ") {
		t.Errorf("empty endpoint must not produce a dangling instance reference; got: %s", err.Error())
	}
}

// TestArtifactKindLookupAsksMoreThanOneInstance pins that an unknown kind is
// not accepted on the word of a single repository instance.
//
// Discovery selects ONE instance per resolution and consecutive resolutions
// rotate across the registered set. An instance on a freshly-joined node
// registers before it has synced its artifact index, so a lookup routed there
// sees nothing. Observed 2026-08-23: `desired set` refused dns, ai-watcher and
// authentication as having "no published version with a resolvable kind" while
// `pkg info dns` reported kind SERVICE on every one of six consecutively
// resolved endpoints.
//
// The bound matters as much as the retry: a genuinely unpublished package must
// still be refused, so the loop is finite and the gate still fails closed.
func TestArtifactKindLookupAsksMoreThanOneInstance(t *testing.T) {
	if artifactKindLookupAttempts < 2 {
		t.Fatalf("a single instance must not settle the question; attempts=%d",
			artifactKindLookupAttempts)
	}
	if artifactKindLookupAttempts > 5 {
		t.Errorf("the retry must stay bounded so an unpublished package is still "+
			"refused promptly; attempts=%d", artifactKindLookupAttempts)
	}
}

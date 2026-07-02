package main

// append_to_ledger_repair_test.go — Phase 33.
//
// Pins the third immutability gate's repair-awareness:
// appendToLedger is the RESOLVER-VISIBLE step in the repair lane.
// getExactRelease and getPublishedDigest read from the ledger, so a
// successful repair that doesn't update the ledger leaves the new bytes
// orphaned and the phantom canonical — exactly the partial-repair state
// observed live on globule-ryzen 2026-06-03 after the Phase 31/32 repair.
//
// These tests cover every gate the ledger-repair path must enforce, plus
// the cross-gate end-to-end test that proves no hidden fourth immutability
// gate blocks the repair after Phase 33.

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const ledgerPhantomDigest = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
const ledgerPhantomBuildID = "phantom-bid-aaaa"
const ledgerNewDigest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
const ledgerNewBuildID = "real-bid-bbbb"

// seedLedgerPhantom writes a single (version, platform) entry to the ledger
// representing a sealed phantom — the state the live cluster reached after
// the Phase 30 partial repair.
func seedLedgerPhantom(t *testing.T, srv *server) {
	t.Helper()
	// Use appendToLedger itself with repair=nil — this is the legitimate
	// "first publish" path, exactly how a real phantom got into the ledger.
	if err := srv.appendToLedger(context.Background(),
		"core@globular.io", "node-agent", "1.2.143",
		ledgerPhantomBuildID, ledgerPhantomDigest, "linux_amd64", 1000,
		nil,
	); err != nil {
		t.Fatalf("seed phantom into ledger: %v", err)
	}
}

func TestLedger_NoRepair_SameVersionDifferentBuildID_Rejects(t *testing.T) {
	// Regression: the default contract is unchanged. Same (version,
	// platform) with a different build_id is rejected. This is exactly
	// what bit Phase 30's first repair attempt (silently downgraded to
	// Warn in completePublish — Phase 33 also makes it fatal for repair).
	srv := newTestServer(t)
	seedLedgerPhantom(t, srv)

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "node-agent", "1.2.143",
		ledgerNewBuildID, ledgerNewDigest, "linux_amd64", 1100,
		nil,
	)
	if err == nil {
		t.Fatal("expected reject, got nil")
	}
	if !strings.Contains(err.Error(), "already published") {
		t.Errorf("expected immutability error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--unseal-official") {
		t.Errorf("error must direct operators to the repair lane; got: %v", err)
	}
}

func TestLedger_RepairValidReplaces(t *testing.T) {
	// The unblock path: ledger entry REPLACED in place. After this:
	//   getExactRelease(...) returns the new build_id.
	//   getPublishedDigest(...) returns the new digest.
	// This is the resolver-visible step.
	srv := newTestServer(t)
	seedLedgerPhantom(t, srv)

	repair := &RepairAuthorization{
		Requested:   true,
		Reason:      "v1.2.131 bytes mislabeled — Path B investigation",
		PriorDigest: ledgerPhantomDigest,
	}
	if err := srv.appendToLedger(context.Background(),
		"core@globular.io", "node-agent", "1.2.143",
		ledgerNewBuildID, ledgerNewDigest, "linux_amd64", 1100,
		repair,
	); err != nil {
		t.Fatalf("ledger repair-unseal failed: %v", err)
	}
	if !repair.Used {
		t.Error("repair.Used not set — post-success audit will skip")
	}
	if repair.PriorBuildID != ledgerPhantomBuildID {
		t.Errorf("repair.PriorBuildID=%q want %q", repair.PriorBuildID, ledgerPhantomBuildID)
	}
	// Verify the resolver-visible state has actually flipped.
	if got := srv.getExactRelease(context.Background(),
		"core@globular.io", "node-agent", "1.2.143", "linux_amd64"); got != ledgerNewBuildID {
		t.Errorf("after repair, getExactRelease=%q want %q (resolver still sees phantom)", got, ledgerNewBuildID)
	}
	if got := srv.getPublishedDigest(context.Background(),
		"core@globular.io", "node-agent", "1.2.143", "linux_amd64"); got != ledgerNewDigest {
		t.Errorf("after repair, getPublishedDigest=%q want %q", got, ledgerNewDigest)
	}
}

func TestLedger_RepairWrongPriorDigest_RejectsFailedPrecondition(t *testing.T) {
	// Same FailedPrecondition shape as gates 1 and 3 — consistent UX so
	// operators see the same error class regardless of which gate fired.
	srv := newTestServer(t)
	seedLedgerPhantom(t, srv)

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "node-agent", "1.2.143",
		ledgerNewBuildID, ledgerNewDigest, "linux_amd64", 1100,
		&RepairAuthorization{
			Requested:   true,
			Reason:      "test",
			PriorDigest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", // wrong
		},
	)
	if err == nil {
		t.Fatal("expected FailedPrecondition, got nil")
	}
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Errorf("code=%v want FailedPrecondition", code)
	}
	if !strings.Contains(err.Error(), "prior-digest mismatch") {
		t.Errorf("error must explain prior-digest mismatch; got: %v", err)
	}
}

func TestLedger_RepairEmptyReason_RejectsInvalidArgument(t *testing.T) {
	srv := newTestServer(t)
	seedLedgerPhantom(t, srv)

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "node-agent", "1.2.143",
		ledgerNewBuildID, ledgerNewDigest, "linux_amd64", 1100,
		&RepairAuthorization{
			Requested:   true,
			Reason:      "",
			PriorDigest: ledgerPhantomDigest,
		},
	)
	if err == nil {
		t.Fatal("expected InvalidArgument, got nil")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code=%v want InvalidArgument", code)
	}
}

func TestLedger_RepairWithoutUnsealFlag_RejectsAsBefore(t *testing.T) {
	// RepairAuthorization with Requested=false is functionally nil —
	// the ledger gate falls through to the default immutability error.
	srv := newTestServer(t)
	seedLedgerPhantom(t, srv)

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "node-agent", "1.2.143",
		ledgerNewBuildID, ledgerNewDigest, "linux_amd64", 1100,
		&RepairAuthorization{
			Requested:   false, // <-- the flag that gates the bypass
			Reason:      "test",
			PriorDigest: ledgerPhantomDigest,
		},
	)
	if err == nil {
		t.Fatal("expected reject, got nil")
	}
	if !strings.Contains(err.Error(), "already published") {
		t.Errorf("expected default immutability error, got: %v", err)
	}
}

func TestLedger_RepairUpdatesLatestBuildID(t *testing.T) {
	// If the phantom was the ledger's LatestBuildID anchor, repair must
	// also update that pointer — otherwise ledger summary queries still
	// return the phantom build_id even after the entry is replaced.
	srv := newTestServer(t)
	seedLedgerPhantom(t, srv)

	// Confirm precondition: phantom IS the latest.
	pre := srv.readLedger(context.Background(), "core@globular.io", "node-agent")
	if pre == nil || pre.LatestBuildID != ledgerPhantomBuildID {
		t.Fatalf("precondition: LatestBuildID=%q want %q", pre.LatestBuildID, ledgerPhantomBuildID)
	}

	if err := srv.appendToLedger(context.Background(),
		"core@globular.io", "node-agent", "1.2.143",
		ledgerNewBuildID, ledgerNewDigest, "linux_amd64", 1100,
		&RepairAuthorization{
			Requested:   true,
			Reason:      "test latest-anchor update",
			PriorDigest: ledgerPhantomDigest,
		},
	); err != nil {
		t.Fatalf("ledger repair failed: %v", err)
	}

	post := srv.readLedger(context.Background(), "core@globular.io", "node-agent")
	if post.LatestBuildID != ledgerNewBuildID {
		t.Errorf("LatestBuildID=%q want %q (anchor not flipped to new build)", post.LatestBuildID, ledgerNewBuildID)
	}
}

func TestLedger_RepairAuditCrossGateState(t *testing.T) {
	// Cross-gate end-to-end pin: the SAME *RepairAuthorization pointer
	// flows through gates 1, 3, and the ledger gate. After all three,
	// repair.Used must be true and repair.PriorBuildID populated. This
	// is the contract the post-success audit in artifact_handlers.go
	// reads to decide whether to fire pkg.repair_unseal.
	srv := newTestServer(t)
	ctx := context.Background()

	// Seed a phantom into BOTH the publish ledger (for getExactRelease)
	// and the storage manifest (for getPublishedDigest) by reusing
	// the seed helpers from the Phase 31/32 tests.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "core@globular.io",
			Name:        "test-svc",
			Version:     "2.0.0",
			Platform:    "linux_amd64",
			Kind:        repopb.ArtifactKind_SERVICE,
		},
		BuildNumber: 1,
		BuildId:     "phantom-cross-gate",
		Checksum:    "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		SizeBytes:   500,
	})

	repair := &RepairAuthorization{
		Requested:   true,
		Reason:      "cross-gate audit-state pin",
		PriorDigest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	// Gate 1: seal.
	if err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "test-svc", "2.0.0", "linux_amd64",
		"sha256:1111111111111111111111111111111111111111111111111111111111111111",
		repopb.ArtifactChannel_STABLE, repair,
	); err != nil {
		t.Fatalf("gate 1 (seal): %v", err)
	}
	// Gate 3: version-immutability.
	if _, err := srv.resolveVersionIntent(ctx,
		"core@globular.io", "test-svc", "linux_amd64",
		repopb.VersionIntent_EXACT, "2.0.0",
		repopb.ArtifactChannel_STABLE, repair,
	); err != nil {
		t.Fatalf("gate 3 (version): %v", err)
	}
	// Gate (new): ledger.
	if err := srv.appendToLedger(ctx,
		"core@globular.io", "test-svc", "2.0.0",
		"real-cross-gate", "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"linux_amd64", 500,
		repair,
	); err != nil {
		t.Fatalf("ledger gate: %v", err)
	}

	if !repair.Used {
		t.Error("repair.Used=false after three gates — post-success audit will skip")
	}
	if repair.PriorBuildID == "" {
		t.Error("repair.PriorBuildID empty after three gates")
	}
}

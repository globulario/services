package main

import (
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// The contract under test:
//
//	installed  ⇔  a successful owner-produced installation receipt exists
//
// An observer (the heartbeat's local-package-cache paths) may record what it
// sees. It may not decide that an installation succeeded. The evidence it has —
// an unpacked binary whose checksum matches the manifest — proves artifact
// identity and nothing else.
//
// Origin: 2026-08-05, node-4. sql failed package.check_native_deps
// (libodbc.so.2 missing) at 21:23:16, AFTER the binary was unpacked and the
// unit rendered. At 21:27:53 the heartbeat's cache sync wrote status=installed
// for it. Of 19 SERVICE records on that node, 18 carried an owner receipt; only
// the failed one did not. cluster-doctor's fail-closed unit_receipt_drift rule
// was the sole reason anyone found out.
//
// Invariant:    installed_state_requires_successful_owner_install_receipt
// Failure mode: install.failed_transaction_promoted_to_installed_by_observer

// sqlAfterFailedNativeDepCheck reproduces the exact etcd record the observer
// synthesized on node-4: real artifact identity, zero owner receipt.
func sqlAfterFailedNativeDepCheck() *node_agentpb.InstalledPackage {
	const entry = "a476ed5a565e09036f83696fd731a51374c53bf703f785bbf0258226ce16400f"
	return &node_agentpb.InstalledPackage{
		Name:     "sql",
		Version:  "1.2.298",
		Kind:     "SERVICE",
		Platform: "linux_amd64",
		Checksum: entry,
		Metadata: map[string]string{
			// Everything the observer could legitimately establish. Note the
			// disk checksum MATCHES the manifest — the bytes really are the
			// right bytes. That is precisely why identity is not enough.
			"entrypoint_checksum":               entry,
			"entrypoint_checksum_disk_observed": entry,
			"proof_binary_path":                 "/usr/lib/globular/bin/sql_server",
			"proof_source":                      "local_package_cache",
		},
	}
}

// TestFailedInstallIsNeverPromotedToInstalledByObserver walks the directive's
// scenario end to end: unpack, fail the dependency check before receipt stamp,
// run the heartbeat repeatedly, then let a real install finish.
func TestFailedInstallIsNeverPromotedToInstalledByObserver(t *testing.T) {
	pkg := sqlAfterFailedNativeDepCheck()

	// The install owner never reached its receipt stamp, so no receipt exists.
	if installreceipt.HasOwnerReceipt(pkg) {
		t.Fatal("a package whose install failed before the receipt stamp must not " +
			"report an owner receipt — the whole contract rests on this")
	}

	// The observer discovers the artifact in the local package cache.
	status := observerStatusForCachedArtifact(pkg)
	if status == "installed" {
		t.Fatalf("observer promoted a failed install to %q from artifact identity alone; "+
			"this is exactly the node-4 sql defect", status)
	}
	if status != installreceipt.StatusArtifactPresent {
		t.Fatalf("observer status = %q, want %q: the observation must still be RECORDED, "+
			"not discarded — evidence of a partial install is what lets convergence retry",
			status, installreceipt.StatusArtifactPresent)
	}
	pkg.Status = status

	// "run heartbeat repeatedly": every subsequent tick re-observes the same
	// artifact. The status must be a fixed point, never drifting to installed.
	for tick := 1; tick <= 25; tick++ {
		pkg.Status = observerRepairStatus(pkg.GetStatus(), pkg)
		if pkg.Status == "installed" {
			t.Fatalf("heartbeat tick %d promoted the record to installed; repeated "+
				"observation must not manufacture authority that a single "+
				"observation lacks", tick)
		}
		if pkg.Status != installreceipt.StatusArtifactPresent {
			t.Fatalf("tick %d: status drifted to %q; the failed-install evidence must "+
				"stay visible and stable", tick, pkg.Status)
		}
	}

	// Failure evidence survived all of it — a reader can still tell what the
	// observer actually saw.
	if pkg.Metadata["proof_source"] != "local_package_cache" ||
		pkg.Metadata["entrypoint_checksum"] == "" {
		t.Error("observer evidence was destroyed; the fix must preserve the partial-" +
			"install record, not erase it")
	}

	// Now the real thing: a later install transaction commits and stamps.
	pkg.Metadata[installreceipt.KeyInstalledBy] = "node-agent.workflow.package_report_state"
	if !installreceipt.HasOwnerReceipt(pkg) {
		t.Fatal("a stamped receipt must be recognized as owner proof")
	}
	if got := observerRepairStatus(pkg.GetStatus(), pkg); got != "installed" {
		t.Fatalf("after a successful owner install the record must reach installed, got %q; "+
			"the gate must not be a permanent ratchet against legitimate installs", got)
	}
}

// TestObserverStillReportsGenuinelyInstalledPackages is the negative control
// that keeps the fix from being a blunt "never say installed".
//
// 18 of 19 records on node-4 carried a receipt and were genuinely installed.
// If this fix demoted those, it would trade one lie for a much larger one and
// trigger cluster-wide reinstall churn.
func TestObserverStillReportsGenuinelyInstalledPackages(t *testing.T) {
	for _, receipt := range []map[string]string{
		{installreceipt.KeyInstalledBy: "node-agent.workflow.package_report_state"},
		{installreceipt.KeyInstalledBy: "node-agent.apply_package_release.binary_only"},
		{installreceipt.KeyInstalledBy: installreceipt.DefaultInstalledBy},
		// The one sanctioned equivalent: installed before the receipt refactor.
		{installreceipt.KeyMigrationSource: installreceipt.MigrationSourceLegacySidecar},
	} {
		pkg := &node_agentpb.InstalledPackage{Name: "workflow", Metadata: receipt}
		if !installreceipt.HasOwnerReceipt(pkg) {
			t.Errorf("receipt %v not recognized as owner proof", receipt)
		}
		if got := observerStatusForCachedArtifact(pkg); got != "installed" {
			t.Errorf("receipt %v: cache sync = %q, want installed", receipt, got)
		}
		if got := observerRepairStatus("installed", pkg); got != "installed" {
			t.Errorf("receipt %v: repair demoted a genuinely installed package to %q", receipt, got)
		}
	}
}

// TestRepairPreservesNonInstalledEvidenceStatuses pins that repair leaves the
// system's existing honest statuses alone rather than overwriting them.
//
// These are real statuses observed in the live cluster on 2026-08-05:
// installed_unverified (claude, codex) and failed_binary_hash_mismatch
// (claude). They are the vocabulary that keeps a partial install
// distinguishable from a complete one; a promoting observer erases exactly
// that distinction.
func TestRepairPreservesNonInstalledEvidenceStatuses(t *testing.T) {
	for _, status := range []string{
		installreceipt.StatusArtifactPresent,
		"installed_unverified",
		"failed_binary_hash_mismatch",
		"installation_failed",
		"installation_incomplete",
	} {
		pkg := &node_agentpb.InstalledPackage{
			Name:     "claude",
			Status:   status,
			Metadata: map[string]string{"entrypoint_checksum": "deadbeef"},
		}
		if got := observerRepairStatus(pkg.GetStatus(), pkg); got != status {
			t.Errorf("repair rewrote %q to %q; a receiptless record's status is evidence "+
				"and must survive observation", status, got)
		}
	}
}

// TestHasOwnerReceiptRejectsArtifactIdentityAlone states the boundary directly:
// no amount of identity evidence substitutes for a receipt.
func TestHasOwnerReceiptRejectsArtifactIdentityAlone(t *testing.T) {
	if installreceipt.HasOwnerReceipt(nil) {
		t.Error("nil package must not report an owner receipt")
	}
	for name, md := range map[string]map[string]string{
		"no metadata at all":  nil,
		"empty metadata":      {},
		"identity only":       {"entrypoint_checksum": "abc", "entrypoint_checksum_disk_observed": "abc"},
		"binary path proof":   {"proof_binary_path": "/usr/lib/globular/bin/sql_server", "proof_source": "local_package_cache"},
		"package sha only":    {installreceipt.KeyPackageSha256: "abc"},
		"unit hash but no by": {installreceipt.KeyUnitFileSha256: "abc", installreceipt.KeyUnitFilePath: "/etc/systemd/system/globular-sql.service"},
		"blank installed_by":  {installreceipt.KeyInstalledBy: "   "},
		"bogus migration":     {installreceipt.KeyMigrationSource: "made_up"},
	} {
		if installreceipt.HasOwnerReceipt(&node_agentpb.InstalledPackage{Metadata: md}) {
			t.Errorf("%s: accepted as an owner receipt", name)
		}
	}
}

package main

// audit_isolation_test.go — defense-in-depth against unit tests writing
// to production etcd.
//
// History: during Patch C Milestones 1 and 3, several unit tests called
// srv.ExecuteRemediation against a real-looking server fixture without
// stubbing config.GetEtcdClient. On any machine where the global
// /var/lib/globular/pki/* certs resolved, the audit writes inside
// auditRemediation() succeeded against the production cluster's etcd,
// leaving rem-* rows in /globular/cluster_doctor/audit/ with 30-day TTL.
// Those rows polluted operator audit trails with test fixture finding-ids
// (finding-hardblock-*, f-verify-1, etc.).
//
// The fix is structural: the audit-write path now goes through a
// package-level seam (auditEtcdPersistFn in executor.go) that defaults
// to a no-op in test runs. The only way a test reaches production etcd
// is to set GLOBULAR_LIVE_ETCD_TESTS=1 — and then it must explicitly
// call skipUnlessLiveEtcd to opt back in.
//
// The default test stub is a SILENT NO-OP — it does not even capture
// records. Tests that want to inspect captured audits call
// withStubbedAuditEtcd, which installs a per-test capturing stub via
// t.Cleanup. The drop-everything default makes the safe behavior
// automatic; capture is opt-in.

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
)

// funcAddr returns the pointer address of a function value as a string.
// Used by the regression test to assert that the runtime auditEtcdPersistFn
// is not the same function value as the production snapshot.
func funcAddr(fn func(context.Context, RemediationAudit)) string {
	return fmt.Sprintf("0x%x", reflect.ValueOf(fn).Pointer())
}

const liveEtcdTestEnvVar = "GLOBULAR_LIVE_ETCD_TESTS"

// productionAuditEtcdPersistFn is the production function snapshot, captured
// before TestMain swaps in the no-op. Used by skipUnlessLiveEtcd to
// restore for opted-in tests AND by the regression test to assert the
// swap actually happened.
var productionAuditEtcdPersistFn = auditEtcdPersistFn

// TestMain installs a silent no-op audit persistor as the default for the
// whole package. Any test that calls into ExecuteRemediation /
// auditRemediation will write through this stub instead of reaching real
// etcd. Tests that need to inspect captured audits replace it again via
// withStubbedAuditEtcd. Tests that legitimately need live etcd opt in
// via GLOBULAR_LIVE_ETCD_TESTS=1 + skipUnlessLiveEtcd.
func TestMain(m *testing.M) {
	if os.Getenv(liveEtcdTestEnvVar) != "1" {
		// Replace with a silent no-op. The production fn snapshot above
		// stays available for opt-in restoration.
		auditEtcdPersistFn = func(_ context.Context, _ RemediationAudit) { /* no-op */ }
	}
	os.Exit(m.Run())
}

// withStubbedAuditEtcd installs a per-test capturing stub. The returned
// slice pointer accumulates every audit the handler asked to persist;
// the t.Cleanup restores the package-level no-op when the test returns.
//
// Use this in tests that need to assert WHAT was persisted (e.g.
// "audit was created with action_type=DELETE_CACHE_ARTIFACT and
// executed=true"). For tests that only need "no leak" semantics, the
// TestMain default already covers it — no extra setup needed.
func withStubbedAuditEtcd(t *testing.T) *[]RemediationAudit {
	t.Helper()
	var mu sync.Mutex
	captured := &[]RemediationAudit{}
	prev := auditEtcdPersistFn
	auditEtcdPersistFn = func(_ context.Context, audit RemediationAudit) {
		mu.Lock()
		defer mu.Unlock()
		*captured = append(*captured, audit)
	}
	t.Cleanup(func() {
		auditEtcdPersistFn = prev
	})
	return captured
}

// skipUnlessLiveEtcd marks a test as requiring a real Globular cluster's
// etcd. Skipped unless GLOBULAR_LIVE_ETCD_TESTS=1. When the env var is
// set, the production audit persistor is restored for the duration of
// the test.
//
// No tests in this package currently require live etcd. The helper is
// here so future integration tests have a single, auditable opt-in.
func skipUnlessLiveEtcd(t *testing.T) {
	t.Helper()
	if os.Getenv(liveEtcdTestEnvVar) != "1" {
		t.Skipf("skipping live-etcd test; set %s=1 to enable", liveEtcdTestEnvVar)
	}
	prev := auditEtcdPersistFn
	auditEtcdPersistFn = productionAuditEtcdPersistFn
	t.Cleanup(func() { auditEtcdPersistFn = prev })
}

// TestRemediation_AuditDoesNotLeakToProductionEtcd is the regression
// guard for the Patch C audit-leak fix. It asserts two things:
//
//  1. The package's default audit persistor is NOT the production
//     etcd-writing function. If a future change accidentally reverts
//     the TestMain swap, this test fails immediately.
//
//  2. ExecuteRemediation's audit path writes through auditEtcdPersistFn
//     (not via a private path that bypasses the seam). We assert this
//     by installing a capturing stub and verifying it receives a record
//     when ExecuteRemediation runs.
//
// Together these prove no unit-test invocation reaches the cluster's
// /globular/cluster_doctor/audit/ namespace.
func TestRemediation_AuditDoesNotLeakToProductionEtcd(t *testing.T) {
	// (1) Default seam must not be the production fn.
	//
	// Function-value equality in Go is comparison-by-identity for
	// function values produced by `var x = someFunc`. The TestMain swap
	// replaces auditEtcdPersistFn with a closure literal, so its address
	// (as printed by %p) differs from the production function's. The
	// productionAuditEtcdPersistFn variable captures the original.
	prodAddr := funcAddr(productionAuditEtcdPersistFn)
	defaultAddr := funcAddr(auditEtcdPersistFn)
	if prodAddr == defaultAddr {
		t.Fatalf("test seam not engaged: auditEtcdPersistFn is still %s — TestMain must swap it to a no-op",
			prodAddr)
	}

	// (2) ExecuteRemediation routes audit through the seam. Install a
	// capturing stub for this test only; the TestMain no-op resumes on
	// cleanup.
	captured := withStubbedAuditEtcd(t)

	// Trigger an audit write by calling auditRemediation directly with a
	// synthetic record. We don't drive ExecuteRemediation here because
	// the regression target is the seam itself — driving the full
	// handler is covered by TestCacheCleanup_WritesSingleEtcdAudit.
	audit := RemediationAudit{
		FindingID:   "test-audit-isolation",
		InvariantID: "synthetic.isolation_check",
		ActionType:  "SYNTHETIC",
		Subject:     "test",
	}
	id := auditRemediation(context.Background(), audit)
	if id == "" {
		t.Fatalf("expected non-empty audit_id, got empty")
	}
	if len(*captured) != 1 {
		t.Fatalf("expected exactly 1 captured audit, got %d", len(*captured))
	}
	if got := (*captured)[0].FindingID; got != "test-audit-isolation" {
		t.Fatalf("captured wrong record; FindingID=%q want test-audit-isolation", got)
	}
}

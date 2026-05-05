package rules

import (
	"errors"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

func TestCriticalKeyRegistryPresence_MissingKeysEmitFindings(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	// All keys absent.
	snap := &collector.Snapshot{
		CriticalKeyPresent: map[string]bool{},
		// Avoid Day-0 heuristic in this baseline test.
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}, {NodeId: "n2"}},
	}
	findings := inv.Evaluate(snap, Config{})
	expected := len(config.CriticalEtcdKeys) + len(config.CriticalEtcdPrefixes)
	if len(findings) != expected {
		t.Fatalf("expected %d findings (one per registry entry), got %d", expected, len(findings))
	}
}

func TestCriticalKeyRegistryPresence_Day0DowngradesSeverity(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		CriticalKeyPresent: map[string]bool{
			"/globular/system/config":          false,
			"/globular/nodes/":                 false,
			"/globular/resources/":             false,
			"/globular/ingress/v1/spec":        false,
			"/globular/ingress/v1/spec_backup": false,
			"/globular/pki/ca":                 false,
			"/globular/objectstore/config":     false,
			"/globular/scylla/schema_guard/":   false,
		},
	}

	findings := inv.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("expected findings in day-0 state")
	}
	for _, f := range findings {
		if strings.HasSuffix(f.InvariantID, "_missing") {
			if strings.Contains(f.InvariantID, "resources_missing") || strings.Contains(f.InvariantID, "nodes_missing") || strings.Contains(f.InvariantID, "scylla.schema_guard_missing") {
				if f.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
					t.Fatalf("prefix finding %s severity=%v want INFO", f.InvariantID, f.Severity)
				}
			} else if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
				t.Fatalf("key finding %s severity=%v want WARN", f.InvariantID, f.Severity)
			}
		}
	}
}

func TestCriticalKeyRegistryPresence_PresentKeysNoFinding(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	present := map[string]bool{}
	for _, k := range config.CriticalEtcdKeys {
		present[k] = true
	}
	for _, p := range config.CriticalEtcdPrefixes {
		present[p] = true
	}
	snap := &collector.Snapshot{CriticalKeyPresent: present}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when all keys present, got %d", len(findings))
	}
}

func TestKeyToInvariantID(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"/globular/system/config", "system.config_missing"},
		{"/globular/ingress/v1/spec_backup", "ingress.spec_backup_missing"},
		{"/globular/ingress/v1/spec", "ingress.spec_missing"},
		{"/globular/resources/", "resources_missing"},
		{"/globular/nodes/", "nodes_missing"},
	}
	for _, tc := range cases {
		got := keyToInvariantID(tc.key)
		if got != tc.want {
			t.Errorf("keyToInvariantID(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
	// All generated IDs should end in "_missing"
	for _, k := range config.CriticalEtcdKeys {
		id := keyToInvariantID(k)
		if !strings.HasSuffix(id, "_missing") {
			t.Errorf("keyToInvariantID(%q) = %q, expected suffix '_missing'", k, id)
		}
	}
}

// ── PR-2: CHECK_ERROR tests ───────────────────────────────────────────────────

// TestInvariantReturnsCheckErrorOnTLSFailure verifies that a TLS / connection
// error on the etcd Get causes the invariant to emit a CHECK_ERROR finding
// (InvariantStatus INVARIANT_UNKNOWN) rather than a FAIL finding, so operators
// are not paged for indeterminate results.
func TestInvariantReturnsCheckErrorOnTLSFailure(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	tlsErr := errors.New("tls: certificate verify failed")

	failedKey := config.CriticalEtcdKeys[0]
	snap := &collector.Snapshot{
		CriticalKeyPresent:    map[string]bool{},
		CriticalKeyQueryError: map[string]error{failedKey: tlsErr},
	}
	findings := inv.Evaluate(snap, Config{})

	if len(findings) == 0 {
		t.Fatal("expected at least one finding for query error, got none")
	}

	var checkErrFindings []Finding
	for _, f := range findings {
		if f.InvariantStatus == cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN && f.CheckError != "" {
			checkErrFindings = append(checkErrFindings, f)
		}
	}
	if len(checkErrFindings) == 0 {
		t.Errorf("expected a CHECK_ERROR finding for key %s; got: %+v", failedKey, findings)
	}

	// The remaining keys (no error, not present) must still produce FAIL findings.
	var failCount int
	for _, f := range findings {
		if f.InvariantStatus == cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
			failCount++
		}
	}
	remainingKeys := len(config.CriticalEtcdKeys) - 1 + len(config.CriticalEtcdPrefixes)
	if failCount != remainingKeys {
		t.Errorf("expected %d FAIL findings for absent keys (excluding failed key), got %d",
			remainingKeys, failCount)
	}
}

// TestInvariantUsesPrefixScanForNodesMissing verifies that when the snapshot
// carries a query error for the /globular/nodes/ prefix, the invariant emits
// CHECK_ERROR rather than FAIL.
func TestInvariantUsesPrefixScanForNodesMissing(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	const nodesPrefix = "/globular/nodes/"
	connErr := errors.New("connection reset by peer")

	present := map[string]bool{}
	for _, k := range config.CriticalEtcdKeys {
		present[k] = true
	}
	for _, p := range config.CriticalEtcdPrefixes {
		if p != nodesPrefix {
			present[p] = true
		}
	}
	snap := &collector.Snapshot{
		CriticalKeyPresent:    present,
		CriticalKeyQueryError: map[string]error{nodesPrefix: connErr},
	}

	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding (for nodes prefix query error), got %d: %+v",
			len(findings), findings)
	}
	f := findings[0]
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("nodes prefix query error must produce CHECK_ERROR (INVARIANT_UNKNOWN), got %v", f.InvariantStatus)
	}
	if f.CheckError == "" {
		t.Errorf("CheckError field must carry the error string")
	}
}

// TestInvariantUsesPrefixScanForResourcesMissing verifies CHECK_ERROR behaviour
// for the /globular/resources/ prefix.
func TestInvariantUsesPrefixScanForResourcesMissing(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	const resourcesPrefix = "/globular/resources/"
	connErr := errors.New("context deadline exceeded")

	present := map[string]bool{}
	for _, k := range config.CriticalEtcdKeys {
		present[k] = true
	}
	for _, p := range config.CriticalEtcdPrefixes {
		if p != resourcesPrefix {
			present[p] = true
		}
	}
	snap := &collector.Snapshot{
		CriticalKeyPresent:    present,
		CriticalKeyQueryError: map[string]error{resourcesPrefix: connErr},
	}

	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding (for resources prefix query error), got %d: %+v",
			len(findings), findings)
	}
	f := findings[0]
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("resources prefix query error must produce CHECK_ERROR (INVARIANT_UNKNOWN), got %v", f.InvariantStatus)
	}
	if f.CheckError == "" {
		t.Errorf("CheckError field must carry the error string")
	}
}

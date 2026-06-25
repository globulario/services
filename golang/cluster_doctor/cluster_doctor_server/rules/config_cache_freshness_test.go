package rules

import (
	"testing"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestServiceConfigCacheFresh proves the OT-3 consumer: when the doctor's
// service-config mirror has not refreshed within the staleness threshold it emits a
// WARN finding; when fresh, or not yet populated, it stays silent.
func TestServiceConfigCacheFresh(t *testing.T) {
	old := serviceConfigCacheLastFresh
	t.Cleanup(func() { serviceConfigCacheLastFresh = old })

	// Stale → one WARN finding.
	serviceConfigCacheLastFresh = func() (time.Time, bool) {
		return time.Now().Add(-2 * time.Minute), true
	}
	f := serviceConfigCacheFresh{}.Evaluate(nil, Config{})
	if len(f) != 1 {
		t.Fatalf("stale mirror: expected 1 finding, got %d", len(f))
	}
	if f[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("stale mirror: expected WARN severity, got %v", f[0].Severity)
	}

	// Fresh → no finding.
	serviceConfigCacheLastFresh = func() (time.Time, bool) { return time.Now(), true }
	if f := (serviceConfigCacheFresh{}).Evaluate(nil, Config{}); len(f) != 0 {
		t.Errorf("fresh mirror: expected no finding, got %d", len(f))
	}

	// Cache not populated → no finding (nothing to judge).
	serviceConfigCacheLastFresh = func() (time.Time, bool) { return time.Time{}, false }
	if f := (serviceConfigCacheFresh{}).Evaluate(nil, Config{}); len(f) != 0 {
		t.Errorf("unpopulated cache: expected no finding, got %d", len(f))
	}
}

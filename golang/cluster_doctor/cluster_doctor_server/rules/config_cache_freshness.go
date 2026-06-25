// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.config_cache_freshness
// @awareness file_role=doctor_rule_flagging_stale_service_config_mirror
// @awareness risk=low
package rules

import (
	"fmt"
	"strconv"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

// serviceConfigCacheStaleAfter is the age beyond which the doctor's
// service-configuration mirror is treated as stale. The cache TTL is 5s (a healthy
// doctor refreshes it every sweep), so 30s without a successful etcd fetch means
// config reads are failing and the served config no longer reflects current state.
const serviceConfigCacheStaleAfter = 30 * time.Second

// serviceConfigCacheLastFresh is the freshness source, overridable in tests.
var serviceConfigCacheLastFresh = config.ServiceConfigCacheLastFresh

// serviceConfigCacheFresh flags when this doctor's service-configuration mirror is
// stale. GetServicesConfigurations serves stale-if-error data (with a NIL error)
// when etcd config fetches fail, so the doctor's own view of service config can be
// up to the StaleIfError window old with no signal — and the doctor would diagnose
// against stale config believing it authoritative. This rule reads the cache's last
// successful fetch time and emits a WARN finding when the mirror has not refreshed
// within a healthy interval, so a stale config view is reported, not silently
// trusted (OT-3; meta.binding_outlives_evidence_until_invalidated — the doctor's own
// observation honesty).
type serviceConfigCacheFresh struct{}

func (serviceConfigCacheFresh) ID() string       { return "config.cache_stale" }
func (serviceConfigCacheFresh) Category() string { return "observability" }
func (serviceConfigCacheFresh) Scope() string    { return "cluster" }

func (serviceConfigCacheFresh) Evaluate(_ *collector.Snapshot, _ Config) []Finding {
	lastFresh, ok := serviceConfigCacheLastFresh()
	if !ok {
		return nil // cache not populated yet (e.g. fresh start) — nothing to judge
	}
	age := time.Since(lastFresh)
	if age < serviceConfigCacheStaleAfter {
		return nil
	}
	return []Finding{{
		FindingID:   FindingID("config.cache_stale", "", ""),
		InvariantID: "config.cache_stale",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "observability",
		Summary: fmt.Sprintf("service-config mirror is stale: %s since the last successful etcd fetch — "+
			"GetServicesConfigurations is serving stale-if-error data, so the served config may not reflect current state",
			age.Round(time.Second)),
		Evidence: []*cluster_doctorpb.Evidence{kvEvidence("config", "ServiceConfigCacheLastFresh", map[string]string{
			"last_fresh_utc": lastFresh.UTC().Format(time.RFC3339),
			"age_seconds":    strconv.Itoa(int(age.Seconds())),
			"stale_after_s":  strconv.Itoa(int(serviceConfigCacheStaleAfter.Seconds())),
		})},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

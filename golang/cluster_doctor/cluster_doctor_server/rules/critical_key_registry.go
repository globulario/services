package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
)

type criticalKeyRegistryPresence struct{}

func (criticalKeyRegistryPresence) ID() string       { return "critical_state.registry_presence" }
func (criticalKeyRegistryPresence) Category() string { return "control_plane" }
func (criticalKeyRegistryPresence) Scope() string    { return "cluster" }

// Evaluate checks all keys from config.CriticalEtcdKeys and
// config.CriticalEtcdPrefixes against the collected snapshot.
// Any missing key emits an ERROR finding (Case 05: CRITICAL_STATE_REGISTRY_AND_OWNERSHIP).
// A query error (TLS failure, connection reset) emits a CHECK_ERROR finding instead of
// FAIL — the verdict is indeterminate and must not page the on-call operator.
func (criticalKeyRegistryPresence) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	day0Bootstrap := isLikelyDay0Bootstrap(snap)

	for _, key := range config.CriticalEtcdKeys {
		if queryErr, failed := snap.CriticalKeyQueryError[key]; failed {
			if mapCheckErr(queryErr) != InvariantStateCheckError {
				// Defensive fallback: non-nil query errors must always map to CHECK_ERROR.
				queryErr = fmt.Errorf("unexpected check-state mapping for key %s: %w", key, queryErr)
			}
			findings = append(findings, checkErrorFinding(key, queryErr))
			continue
		}
		present, ok := snap.CriticalKeyPresent[key]
		if ok && present {
			continue
		}
		invariant := keyToInvariantID(key)
		severity := cluster_doctorpb.Severity_SEVERITY_ERROR
		summary := fmt.Sprintf("Critical etcd key %s is absent; authoritative owner must restore it.", key)
		if day0Bootstrap {
			severity = cluster_doctorpb.Severity_SEVERITY_WARN
			summary = fmt.Sprintf("Day-0 bootstrap likely in progress: critical etcd key %s not published yet.", key)
		}
		findings = append(findings, Finding{
			FindingID:   FindingID(invariant, "cluster", key),
			InvariantID: invariant,
			Severity:    severity,
			Category:    "control_plane",
			EntityRef:   "cluster",
			Summary:     summary,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", fmt.Sprintf("Get(%s)", key), map[string]string{
					"key":    key,
					"result": "key_not_found",
				}),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	for _, prefix := range config.CriticalEtcdPrefixes {
		if queryErr, failed := snap.CriticalKeyQueryError[prefix]; failed {
			if mapCheckErr(queryErr) != InvariantStateCheckError {
				// Defensive fallback: non-nil query errors must always map to CHECK_ERROR.
				queryErr = fmt.Errorf("unexpected check-state mapping for prefix %s: %w", prefix, queryErr)
			}
			findings = append(findings, checkErrorFinding(prefix, queryErr))
			continue
		}
		present, ok := snap.CriticalKeyPresent[prefix]
		if ok && present {
			continue
		}
		invariant := keyToInvariantID(prefix)
		severity := cluster_doctorpb.Severity_SEVERITY_WARN
		summary := fmt.Sprintf("No keys found under critical prefix %s.", prefix)
		if day0Bootstrap {
			severity = cluster_doctorpb.Severity_SEVERITY_INFO
			summary = fmt.Sprintf("Day-0 bootstrap likely in progress: no keys published yet under critical prefix %s.", prefix)
		}
		findings = append(findings, Finding{
			FindingID:   FindingID(invariant, "cluster", prefix),
			InvariantID: invariant,
			Severity:    severity,
			Category:    "control_plane",
			EntityRef:   "cluster",
			Summary:     summary,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", fmt.Sprintf("Get(%s, prefix)", prefix), map[string]string{
					"prefix": prefix,
					"result": "no_keys_found",
				}),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}

// checkErrorFinding builds a CHECK_ERROR finding for a key whose etcd query
// failed. The InvariantStatus is INVARIANT_UNKNOWN so aggregators know the
// verdict is indeterminate — neither PASS nor FAIL.
func checkErrorFinding(key string, queryErr error) Finding {
	invariant := keyToInvariantID(key)
	return Finding{
		FindingID:   FindingID(invariant+"_check_error", "cluster", key),
		InvariantID: invariant,
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "control_plane",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"Critical key/prefix %s: etcd query failed — verdict is indeterminate (check_error).", key),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", fmt.Sprintf("Get(%s)", key), map[string]string{
				"key":         key,
				"error":       queryErr.Error(),
				"check_state": string(InvariantStateCheckError),
			}),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
		CheckError:      queryErr.Error(),
	}
}

// keyToInvariantID converts an etcd key path to a stable dot-separated invariant ID.
// "/globular/ingress/v1/spec_backup" → "ingress.spec_backup_missing"
func keyToInvariantID(key string) string {
	// Strip leading slash and prefix "/globular/"
	s := strings.TrimPrefix(key, "/")
	s = strings.TrimPrefix(s, "globular/")
	s = strings.TrimSuffix(s, "/")
	// Replace version segments (v1, v2) and slashes with dots.
	parts := strings.Split(s, "/")
	var kept []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		// Skip bare version segments like "v1" that aren't informative in the ID.
		if len(p) <= 3 && p[0] == 'v' && len(p) > 1 {
			continue
		}
		kept = append(kept, strings.ReplaceAll(p, "-", "_"))
	}
	return strings.Join(kept, ".") + "_missing"
}

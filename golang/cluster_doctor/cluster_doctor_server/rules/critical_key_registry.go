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

// criticalKeyOwnershipComplete fires when one or more critical etcd keys lack
// a declared owner in config.CriticalKeyPolicies. This catches any future key
// added to the live-check list without adding the corresponding governance entry.
// The finding is purely static — no etcd query is needed.
//
// Invariant: critical_state.registry_ownership_required
type criticalKeyOwnershipComplete struct{}

func (criticalKeyOwnershipComplete) ID() string       { return "critical_state.ownership_complete" }
func (criticalKeyOwnershipComplete) Category() string { return "control_plane" }
func (criticalKeyOwnershipComplete) Scope() string    { return "cluster" }

func (criticalKeyOwnershipComplete) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if len(snap.CriticalKeyPolicyGaps) == 0 {
		return nil
	}
	gapList := strings.Join(snap.CriticalKeyPolicyGaps, ", ")
	return []Finding{{
		FindingID:   FindingID("critical_state.ownership_complete", "cluster", gapList),
		InvariantID: "critical_state.ownership_complete",
		Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:    "control_plane",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"%d critical etcd key(s) have no declared owner in config.CriticalKeyPolicies: [%s]. "+
				"These keys cannot be governed, restored, or audited. "+
				"Add a CriticalKeyPolicy entry for each key. "+
				"Invariant: critical_state.registry_ownership_required.",
			len(snap.CriticalKeyPolicyGaps), gapList,
		),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("config", "PolicyGapsForKeys(CriticalEtcdKeys, CriticalEtcdPrefixes)", map[string]string{
				"gap_count":   fmt.Sprintf("%d", len(snap.CriticalKeyPolicyGaps)),
				"gap_keys":    gapList,
				"total_keys":  fmt.Sprintf("%d", len(config.CriticalEtcdKeys)+len(config.CriticalEtcdPrefixes)),
				"policy_size": fmt.Sprintf("%d", len(config.CriticalKeyPolicies)),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Add a CriticalKeyPolicy entry to golang/config/critical_keys.go for each key listed above (Owner, SchemaVersion, DeletePolicyName, DoctorInvariant must all be non-empty).",
				"# Edit golang/config/critical_keys.go → CriticalKeyPolicies",
			),
			step(2,
				"Verify TestRegistryKeyHasCompletePolicy passes after adding the entry.",
				"cd golang && go test ./config/... -run TestRegistryKeyHasCompletePolicy",
			),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
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

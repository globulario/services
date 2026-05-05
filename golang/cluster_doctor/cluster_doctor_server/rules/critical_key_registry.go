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
func (criticalKeyRegistryPresence) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding

	for _, key := range config.CriticalEtcdKeys {
		present, ok := snap.CriticalKeyPresent[key]
		if ok && present {
			continue
		}
		// Derive a stable invariant ID from the key path.
		invariant := keyToInvariantID(key)
		findings = append(findings, Finding{
			FindingID:   FindingID(invariant, "cluster", key),
			InvariantID: invariant,
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "control_plane",
			EntityRef:   "cluster",
			Summary:     fmt.Sprintf("Critical etcd key %s is absent; authoritative owner must restore it.", key),
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
		present, ok := snap.CriticalKeyPresent[prefix]
		if ok && present {
			continue
		}
		invariant := keyToInvariantID(prefix)
		findings = append(findings, Finding{
			FindingID:   FindingID(invariant, "cluster", prefix),
			InvariantID: invariant,
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "cluster",
			Summary:     fmt.Sprintf("No keys found under critical prefix %s.", prefix),
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


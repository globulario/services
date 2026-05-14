package rules

// repository_dns_invariants.go — Doctor rules that enforce the two structural
// invariants this change is anchored on:
//
//   1. The repository must never let a build_id referenced by active desired
//      state (Layer 2) be missing, archived, or revoked. If it is, the
//      controller's reconciler will install-storm against a build_id the
//      repository can no longer resolve. (failure_mode:
//      repository.desired_build_id_orphaned)
//
//   2. DNS must not advertise a record for a node that does not actually serve
//      what the record promises. If a record points at a node whose service
//      is planned but not installed, or installed but not running, clients
//      hit dead endpoints for one TTL. (failure_mode: dns.desired_ghost_records)
//
// Each invariant is a single, focused rule. They are deliberately silent on
// healthy steady-state — they emit a Finding only when the join is wrong.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/config"
)

// ─────────────────────────────────────────────────────────────────────────
// repository.desired_build_ids_resolve
// ─────────────────────────────────────────────────────────────────────────

// repositoryDesiredBuildIDsResolve fails when an active desired-state build_id
// has no matching installable artifact in the repository. This is the
// invariant we patched the resolver / purge guard to enforce; the doctor rule
// surfaces the case where the data is already in the broken state (e.g. the
// repository was rebuilt and an old desired-state record still points at a
// build_id that was never re-indexed).
type repositoryDesiredBuildIDsResolve struct{}

func (repositoryDesiredBuildIDsResolve) ID() string       { return "repository.desired_build_ids_resolve" }
func (repositoryDesiredBuildIDsResolve) Category() string { return "repository" }
func (repositoryDesiredBuildIDsResolve) Scope() string    { return "cluster" }

// desiredBuildIDsReader is the indirection that lets tests inject desired-state
// without going through etcd. Production code uses readDesiredBuildIDs.
var desiredBuildIDsReader = readDesiredBuildIDs

func (r repositoryDesiredBuildIDsResolve) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// Two inputs:
	//  (a) the set of build_ids the cluster currently desires
	//  (b) the set of build_ids the repository can resolve (i.e. installable
	//      manifests in the catalog).
	//
	// (a) is collected via desiredBuildIDsReader, which by default reads etcd
	//     directly because the snapshot does not carry a per-build_id desired
	//     index. Tests override this to inject controlled desired-state data.
	//
	// (b) comes from the repository's per-node InstalledBuildIDs in NodeHealth
	//     plus the RepositoryFindings the collector already gathered. We treat
	//     "the repository has an entry for build_id X" as the resolution proof.
	//     We do NOT make a fresh ResolveArtifact RPC from the doctor — that's
	//     the repository's job and would couple the doctor to repo internals.

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	desired := desiredBuildIDsReader(ctx)
	if len(desired) == 0 {
		// Pre-bootstrap or pre-roll-forward — nothing to check.
		return nil
	}

	// Build the set of build_ids the repository has produced (i.e. that any
	// node has ever successfully installed). This is a lower bound but
	// catches the production case where the desired set references a build_id
	// no node has ever seen, indicating the repository lost the entry.
	known := map[string]bool{}
	for _, health := range snap.NodeHealths {
		if health == nil {
			continue
		}
		for _, bid := range health.GetInstalledBuildIds() {
			if strings.TrimSpace(bid) != "" {
				known[bid] = true
			}
		}
	}

	var findings []Finding
	for bid, ref := range desired {
		if known[bid] {
			continue
		}
		findings = append(findings, Finding{
			FindingID:       FindingID("repository.desired_build_ids_resolve", "cluster", bid),
			InvariantID:     "repository.desired_build_ids_resolve",
			Severity:        cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:        "repository",
			EntityRef:       ref,
			Summary:         fmt.Sprintf("DesiredBuildIdOrphaned: %s pins build_id=%s but the repository has no installable artifact for it — installs will fail until repository repair or desired roll-forward", ref, bid),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("etcd", "/globular/resources/*", map[string]string{
					"build_id":     bid,
					"desired_ref":  ref,
					"hint":         "controller dispatched install workflows that the repository cannot serve",
					"forbidden_fix": "do NOT delete the desired build_id — that just hides the breakage; repair the repository index",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Inspect repository: globular repository ls --publisher core@globular.io --name <name>", ""),
				step(2, "Reindex from blobs: globular repository repair-index --from-all --dry-run", ""),
				step(3, "If reindex impossible, roll desired forward to a resolvable build", "globular services desired set <name> <new_version>"),
			},
		})
	}
	return findings
}

// readDesiredBuildIDs scans all desired-state etcd prefixes and returns a map
// build_id → human-readable ref (for evidence). Best-effort: returns empty
// map on any etcd error.
func readDesiredBuildIDs(ctx context.Context) map[string]string {
	out := map[string]string{}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return out
	}
	prefixes := []string{
		"/globular/resources/ServiceDesiredVersion/",
		"/globular/resources/InfrastructureRelease/",
		"/globular/resources/DesiredService/",
		"/globular/resources/ServiceRelease/",
	}
	type genericRec struct {
		Spec *struct {
			BuildID string `json:"build_id"`
		} `json:"spec"`
		Status *struct {
			ResolvedBuildID string `json:"resolved_build_id"`
			BuildID         string `json:"build_id"`
		} `json:"status"`
	}
	for _, prefix := range prefixes {
		resp, getErr := cli.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithLimit(500))
		if getErr != nil {
			continue
		}
		for _, kv := range resp.Kvs {
			var rec genericRec
			if json.Unmarshal(kv.Value, &rec) != nil {
				continue
			}
			ref := string(kv.Key)
			if rec.Status != nil {
				if rec.Status.ResolvedBuildID != "" {
					out[rec.Status.ResolvedBuildID] = ref
				}
				if rec.Status.BuildID != "" {
					out[rec.Status.BuildID] = ref
				}
			}
			if rec.Spec != nil && rec.Spec.BuildID != "" {
				out[rec.Spec.BuildID] = ref
			}
		}
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────
// dns.records_match_runtime_health
// ─────────────────────────────────────────────────────────────────────────

// dnsRecordsMatchRuntimeHealth fails when DNS records point to nodes whose
// promised service is not Installed + Runtime-healthy.
//
// The check is performed inferentially against snapshot data — we do NOT
// dial the DNS server. Per-record validation requires a service that knows
// every published record set, which is the DNS server itself; here we focus
// on the higher-leverage check: identify any node that would still be
// included in a profile-derived record set despite failing the readiness
// gate. If the DNS reconciler is unpatched, those nodes are still published
// and clients will hit them. If the reconciler IS patched, this rule
// silently observes the gating in action — no finding emitted.
type dnsRecordsMatchRuntimeHealth struct{}

func (dnsRecordsMatchRuntimeHealth) ID() string       { return "dns.records_match_runtime_health" }
func (dnsRecordsMatchRuntimeHealth) Category() string { return "dns" }
func (dnsRecordsMatchRuntimeHealth) Scope() string    { return "cluster" }

// dnsCriticalProfileService maps each "profile" that backs a record to the
// SERVICE that must be installed+healthy on a node for that profile's record
// to be safe to publish. Mirrors dnsServiceUnitName in the controller.
var dnsCriticalProfileService = map[string]string{
	"gateway":       "gateway",
	"dns":           "dns",
	"control-plane": "cluster-controller",
	"core":          "dns", // core nodes back dns.<domain> when no dns profile exists
}

func (r dnsRecordsMatchRuntimeHealth) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		health := snap.NodeHealths[nodeID]
		inv := snap.Inventories[nodeID]
		if health == nil || inv == nil {
			continue
		}
		installed := map[string]bool{}
		for name := range health.GetInstalledVersions() {
			installed[strings.ToLower(strings.TrimSpace(name))] = true
		}
		unitState := map[string]string{}
		for _, u := range inv.GetUnits() {
			unitState[strings.TrimSpace(u.GetName())] = strings.ToLower(strings.TrimSpace(u.GetState()))
		}

		for _, profile := range node.GetProfiles() {
			service, ok := dnsCriticalProfileService[strings.ToLower(strings.TrimSpace(profile))]
			if !ok {
				continue
			}
			if !installed[service] {
				findings = append(findings, Finding{
					FindingID:       FindingID("dns.records_match_runtime_health", nodeID, service+":not_installed"),
					InvariantID:     "dns.records_match_runtime_health",
					Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
					Category:        "dns",
					EntityRef:       nodeID + "/" + service,
					Summary:         fmt.Sprintf("Node %s has profile=%s but service %q is not installed — any DNS record gated on this profile/service must withdraw this node", nodeID, profile, service),
					InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
					Evidence: []*cluster_doctorpb.Evidence{
						kvEvidence("cluster_controller", "GetClusterHealthV1+GetInventory", map[string]string{
							"node_id":         nodeID,
							"profile":         profile,
							"service":         service,
							"installed":       "false",
							"forbidden_fix":   "do NOT publish from profile alone; gate on InstalledServices",
							"expected_dns":    "no_desired_ghost_records",
						}),
					},
					Remediation: []*cluster_doctorpb.RemediationStep{
						step(1, "Reconcile to install the service or remove the profile from the node", "globular cluster reconcile"),
						step(2, "Verify DNS reconciler is publishing this node's records as withdrawn", "journalctl -u globular-cluster-controller | grep 'dns reconciler: WITHDREW'"),
					},
				})
				continue
			}
			unit := packageUnit(service)
			if state, ok := unitState[unit]; ok && state != "active" {
				findings = append(findings, Finding{
					FindingID:       FindingID("dns.records_match_runtime_health", nodeID, service+":unhealthy"),
					InvariantID:     "dns.records_match_runtime_health",
					Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
					Category:        "dns",
					EntityRef:       nodeID + "/" + service,
					Summary:         fmt.Sprintf("Node %s has profile=%s and service %q installed, but runtime unit %s state=%s — DNS must not advertise this node", nodeID, profile, service, unit, state),
					InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
					Evidence: []*cluster_doctorpb.Evidence{
						kvEvidence("cluster_controller", "GetClusterHealthV1+GetInventory", map[string]string{
							"node_id":       nodeID,
							"profile":       profile,
							"service":       service,
							"unit":          unit,
							"runtime_state": state,
							"forbidden_fix": "do NOT publish from installed state alone; gate on runtime health",
						}),
					},
					Remediation: []*cluster_doctorpb.RemediationStep{
						step(1, fmt.Sprintf("Inspect unit logs: journalctl -u %s -n 100", unit), ""),
						step(2, "Wait for runtime to recover, or repair the service", "globular cluster reconcile"),
					},
				})
			}
		}
	}
	return findings
}

// ─────────────────────────────────────────────────────────────────────────
// fallback.requires_manifest_checksum (static config check)
// ─────────────────────────────────────────────────────────────────────────

// fallbackRequiresManifestChecksum is a static guard that fires when the
// repository's per-source configuration has its require_checksum policy
// flipped off. The composed-path failure log records the version of this
// bug where the controller tried to fallback to a mirror that accepts
// missing checksums; we don't have to wait for that situation again to
// flag the misconfiguration. The rule is collector-light: it inspects the
// repository operational status the collector already gathers.
type fallbackRequiresManifestChecksum struct{}

func (fallbackRequiresManifestChecksum) ID() string       { return "fallback.requires_manifest_checksum" }
func (fallbackRequiresManifestChecksum) Category() string { return "repository" }
func (fallbackRequiresManifestChecksum) Scope() string    { return "cluster" }

func (f fallbackRequiresManifestChecksum) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// We have no structured "checksum policy off" signal in the snapshot yet.
	// The rule is intentionally a placeholder that fires whenever the
	// RepositoryFindings list explicitly carries a finding whose ID/summary
	// references checksum policy weakening. Adding a structured field would
	// be the next iteration; for now, surface only confirmed-bad evidence so
	// the rule has zero false-positive risk.
	var findings []Finding
	for _, rf := range snap.RepositoryFindings {
		if rf == nil {
			continue
		}
		kind := strings.ToLower(rf.Kind)
		reason := strings.ToLower(rf.Reason)
		if !strings.Contains(kind, "checksum") && !strings.Contains(reason, "checksum") {
			continue
		}
		if !strings.Contains(kind, "require") && !strings.Contains(reason, "require") &&
			!strings.Contains(reason, "policy") && !strings.Contains(reason, "weakened") &&
			!strings.Contains(reason, "disabled") {
			continue
		}
		findings = append(findings, Finding{
			FindingID:       FindingID("fallback.requires_manifest_checksum", "cluster", rf.Kind+":"+rf.ArtifactKey),
			InvariantID:     "fallback.requires_manifest_checksum",
			Severity:        cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:        "repository",
			EntityRef:       "repository/" + rf.ArtifactKey,
			Summary:         fmt.Sprintf("Repository checksum policy weakened: %s — fallback sources must never accept artifacts without a verified manifest checksum", rf.Reason),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "ListRepositoryFindings", map[string]string{
					"finding_kind":  rf.Kind,
					"artifact_key":  rf.ArtifactKey,
					"reason":        rf.Reason,
					"forbidden_fix": "do NOT relax require_checksum=true to silence transient install failures",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Re-enable require_checksum on all configured upstream sources", ""),
				step(2, "Verify mirrors are providing sha256 in their manifest responses", ""),
			},
		})
	}
	return findings
}

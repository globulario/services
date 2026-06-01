// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=repository_status_health_rule
// @awareness implements=globular.platform:intent.repository.lifecycle_state_machine
// @awareness risk=high
package rules

// repository_status.go — doctor invariants driven by GetRepositoryStatus.
//
// The invariant replaces the "cluster.repo.reachable" pending stub now that
// GetRepositoryStatus is live on the repository service. It fires when:
//
//   - The repository service is unreachable (ReachError set)
//   - The service reports mode DEGRADED (optional MinIO mirror down)
//   - The service reports mode READ_ONLY (Scylla down — writes blocked)
//   - The service reports mode LOCAL_ONLY (both Scylla and MinIO down)
//   - A dependency-mode coherence violation is detected (watchdog bug)
//
// Severity ladder:
//
//	DEGRADED   → INFO  — mirror down, reads/writes work against local CAS
//	READ_ONLY  → WARN  — metadata writes blocked, reads still serve
//	LOCAL_ONLY → ERROR — only locally-verified blobs can be served
//	UNREACHABLE → ERROR — cannot prove any guarantee
//	Coherence violations → ERROR (watchdog reporting inconsistent state)

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type repositoryOperationalMode struct{}

func (repositoryOperationalMode) ID() string       { return "repository.operational_mode" }
func (repositoryOperationalMode) Category() string { return "repository" }
func (repositoryOperationalMode) Scope() string    { return "cluster" }

func (repositoryOperationalMode) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil {
		return nil
	}

	// ── Missing etcd registration (post-bootstrap only) ──────────────────────
	// During bootstrap snap.Nodes is empty — we cannot tell whether the
	// repository service simply has not been deployed yet, so we stay silent.
	// Once at least one node has joined, a missing endpoint is actionable.
	if snap.RepositoryEndpointMissing && len(snap.Nodes) > 0 {
		return []Finding{{
			FindingID:   FindingID("repository.endpoint_missing", "cluster", "endpoint_not_found_in_etcd"),
			InvariantID: "repository.endpoint_missing",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "repository",
			EntityRef:   "cluster",
			Summary:     "Repository service not registered in etcd — package delivery unavailable",
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "etcd", map[string]string{
					"service": "repository.PackageRepository",
					"reason":  "endpoint_not_found_in_etcd",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Verify the repository service is running on at least one node", "globular service status repository.PackageRepository"),
				step(2, "Restart repository service if stopped", "systemctl restart globular-repository.service"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		}}
	}

	if snap.RepositoryOperationalStatus == nil {
		// No repository client configured — emit nothing (degraded gracefully).
		return nil
	}
	s := snap.RepositoryOperationalStatus

	// ── Repository unreachable ───────────────────────────────────────────────
	if s.ReachError != nil {
		return []Finding{{
			FindingID:   FindingID("repository.unreachable", "cluster", s.ReachError.Error()),
			InvariantID: "repository.unreachable",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "repository",
			EntityRef:   "cluster",
			Summary:     "Repository service unreachable — GetRepositoryStatus failed",
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "GetRepositoryStatus", map[string]string{
					"error": s.ReachError.Error(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check repository service health", "globular service status repository.PackageRepository"),
				step(2, "Restart repository service if stopped", "systemctl restart globular-repository.service"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		}}
	}

	var findings []Finding

	// ── Operational mode findings ────────────────────────────────────────────
	switch s.Mode {
	case "DEGRADED":
		// MinIO mirror down — reads/writes work against local CAS; mirror writes skipped.
		findings = append(findings, Finding{
			FindingID:   FindingID("repository.degraded_mode", "cluster", s.Reason),
			InvariantID: "repository.degraded_mode",
			Severity:    cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:    "repository",
			EntityRef:   "cluster",
			Summary:     fmt.Sprintf("Repository DEGRADED: %s", degradedReason(s)),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "GetRepositoryStatus", repoStatusEvidence(s)),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check MinIO mirror connectivity", "globular repository status"),
				step(2, "Verify MinIO service is running on storage nodes", "globular cluster health"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})

	case "READ_ONLY":
		// Scylla down — artifact metadata writes are blocked; local reads still work.
		findings = append(findings, Finding{
			FindingID:   FindingID("repository.read_only_mode", "cluster", s.Reason),
			InvariantID: "repository.read_only_mode",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "repository",
			EntityRef:   "cluster",
			Summary:     fmt.Sprintf("Repository READ_ONLY: %s", degradedReason(s)),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "GetRepositoryStatus", repoStatusEvidence(s)),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check ScyllaDB health on all storage nodes", "globular cluster health"),
				step(2, "Verify scylla-server is running", "systemctl status scylla-server"),
				step(3, "Check ScyllaDB ring status", "nodetool status"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})

	case "LOCAL_ONLY":
		// Both Scylla and MinIO down — only locally-verified blobs can be served.
		findings = append(findings, Finding{
			FindingID:   FindingID("repository.local_only_mode", "cluster", s.Reason),
			InvariantID: "repository.local_only_mode",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "repository",
			EntityRef:   "cluster",
			Summary:     "Repository LOCAL_ONLY: both ScyllaDB and MinIO mirror are unavailable",
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("repository", "GetRepositoryStatus", repoStatusEvidence(s)),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Restore ScyllaDB — required for write/query capabilities", "systemctl restart scylla-server"),
				step(2, "Restore MinIO mirror", "systemctl restart globular-minio.service"),
				step(3, "Verify full recovery", "globular repository status"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})

	case "FULL", "":
		// Healthy — no finding.
	}

	// ── Dependency-mode coherence violations ─────────────────────────────────
	// These fire when the watchdog reports a contradiction (e.g. Scylla dep
	// UNAVAILABLE but mode=FULL). Indicates a bug in dep_health.go.
	for _, dep := range s.Dependencies {
		if dep.Status != "UNAVAILABLE" {
			continue
		}
		if dep.Name == "scylladb" && s.Mode == "FULL" {
			findings = append(findings, Finding{
				FindingID:   FindingID("repository.watchdog_inconsistency", "cluster", dep.Name),
				InvariantID: "repository.watchdog_inconsistency",
				Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:    "repository",
				EntityRef:   "cluster",
				Summary:     "Repository watchdog inconsistency: scylladb UNAVAILABLE but mode=FULL",
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("repository", "GetRepositoryStatus", map[string]string{
						"dependency": dep.Name,
						"dep_status": dep.Status,
						"mode":       s.Mode,
						"invariant":  "scylla_unavailable_but_mode_full",
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Restart repository service to re-run dep_health watchdog first check", "systemctl restart globular-repository.service"),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
		if dep.Name == "minio_mirror" && s.Mode == "FULL" {
			// MinIO UNAVAILABLE + mode=FULL is the correct non-degraded case
			// only if minio is present but this contradicts it. Actually if
			// MinIO is UNAVAILABLE, mode must be DEGRADED or worse.
			// If mode=FULL with minio UNAVAILABLE that's a coherence bug.
			findings = append(findings, Finding{
				FindingID:   FindingID("repository.watchdog_inconsistency", "cluster", dep.Name),
				InvariantID: "repository.watchdog_inconsistency",
				Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:    "repository",
				EntityRef:   "cluster",
				Summary:     "Repository watchdog inconsistency: minio_mirror UNAVAILABLE but mode=FULL",
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("repository", "GetRepositoryStatus", map[string]string{
						"dependency": dep.Name,
						"dep_status": dep.Status,
						"mode":       s.Mode,
						"invariant":  "minio_unavailable_but_mode_full",
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, "Restart repository service to re-run dep_health watchdog first check", "systemctl restart globular-repository.service"),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}

	return findings
}

func degradedReason(s *collector.RepositoryOperationalStatus) string {
	if s.Reason != "" {
		return s.Reason
	}
	for _, d := range s.Dependencies {
		if d.Status == "UNAVAILABLE" {
			return d.Name + " unavailable"
		}
	}
	return s.Mode
}

func repoStatusEvidence(s *collector.RepositoryOperationalStatus) map[string]string {
	ev := map[string]string{
		"service": s.Service,
		"mode":    s.Mode,
		"reason":  s.Reason,
	}
	for _, d := range s.Dependencies {
		ev["dep."+d.Name] = d.Status
	}
	for _, c := range s.Capabilities {
		ev["cap."+c.Name] = c.Status
	}
	return ev
}

package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// scyllaManagerEndpoint is the HTTP API endpoint the rule probes. Exposed
// as a var so tests can redirect to an httptest.Server.
//
// scylla-manager listens on the routable IP, port 5080. The rule runs from
// cluster-doctor which is on the same node as scylla-manager in founding-
// quorum topologies. Direct localhost would work for single-node, but the
// configured routable IP is the canonical address the package generates
// (`http: {host}:5080` in scylla-manager.yaml) so we use it consistently.
var scyllaManagerEndpoint = "http://10.0.0.63:5080"

// scyllaManagerHTTPClient is the http client used to probe the manager.
// Short timeout; this rule must not block the snapshot.
var scyllaManagerHTTPClient = &http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
	},
}

// scyllaManagerClusterRegistered fires when:
//   - the globular-scylla-manager.service unit is active on at least one node, and
//   - scylla-manager's /api/v1/clusters endpoint returns an empty array
//
// Project R discovered that scylla-manager 3.10.1 can run "active" while
// having no registered cluster — backups, repairs, and restores all silently
// unavailable. Project S adds the package-side enforcement that registers
// the cluster at install time. This rule is the safety net: it fires when
// the enforcement is missing, has not yet run, or has failed, surfacing the
// "running but unregistered" state as a backup-readiness failure.
type scyllaManagerClusterRegistered struct{}

func (scyllaManagerClusterRegistered) ID() string {
	return "scylla_manager.cluster_registered"
}
func (scyllaManagerClusterRegistered) Category() string { return "infrastructure" }
func (scyllaManagerClusterRegistered) Scope() string    { return "cluster" }

func (r scyllaManagerClusterRegistered) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// 1. Determine whether scylla-manager is active anywhere.
	if !anyNodeRunsScyllaManager(snap) {
		// scylla-manager not active — different rules cover the missing/
		// inactive case. This rule only fires when the daemon IS running
		// but is unconfigured.
		return nil
	}

	// 2. Probe the HTTP API. Short timeout; network failures are silent
	// because they could mask transient issues unrelated to this rule.
	clusters, err := fetchScyllaManagerClusters(context.Background(), scyllaManagerEndpoint)
	if err != nil {
		// Inconclusive — emit no finding. The probe error is logged elsewhere
		// (collector/health probes). This rule should not double-report.
		return nil
	}

	if len(clusters) == 0 {
		return []Finding{newScyllaManagerUnregisteredFinding()}
	}

	return nil
}

// anyNodeRunsScyllaManager reports whether at least one node's inventory
// shows globular-scylla-manager.service in an active state. The collector
// populates the per-node Units list as part of GetInventory.
func anyNodeRunsScyllaManager(snap *collector.Snapshot) bool {
	if snap == nil {
		return false
	}
	for _, inv := range snap.Inventories {
		if inv == nil {
			continue
		}
		for _, u := range inv.GetUnits() {
			if !strings.EqualFold(u.GetName(), "globular-scylla-manager.service") {
				continue
			}
			if isActiveUnitState(u) {
				return true
			}
		}
	}
	return false
}

// isActiveUnitState returns true when systemd reports the unit as active+running.
// Defensive: the inventory uses "active" / "running" but historical entries may
// use upper case or include sub-states like "auto-restart".
func isActiveUnitState(u *node_agentpb.UnitStatus) bool {
	if u == nil {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(u.GetState()))
	return state == "active" || strings.HasPrefix(state, "active")
}

// fetchScyllaManagerClusters issues GET <endpoint>/api/v1/clusters and
// returns the parsed list. Returns an error on network failure, non-2xx
// response, or unparseable body.
func fetchScyllaManagerClusters(ctx context.Context, endpoint string) ([]map[string]any, error) {
	url := strings.TrimRight(endpoint, "/") + "/api/v1/clusters"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := scyllaManagerHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("scylla-manager /api/v1/clusters returned status %d", resp.StatusCode)
	}
	var clusters []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&clusters); err != nil {
		return nil, fmt.Errorf("decode /api/v1/clusters: %w", err)
	}
	return clusters, nil
}

func newScyllaManagerUnregisteredFinding() Finding {
	const id = "scylla_manager.cluster_registered"
	summary := "scylla-manager is running but no Scylla cluster is registered " +
		"(backup, repair, and restore are unavailable until `sctool cluster add` runs)"
	return Finding{
		FindingID:       FindingID(id, "globular-scylla-manager", "no_cluster_registered"),
		InvariantID:     id,
		Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:        "infrastructure",
		EntityRef:       "globular-scylla-manager.service",
		Summary:         summary,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("scylla_manager.http", "GET /api/v1/clusters", map[string]string{
				"endpoint":      scyllaManagerEndpoint,
				"cluster_count": "0",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Run the package-shipped registration script (Project S): "+
					"/usr/lib/globular/bin/scylla-manager-register-cluster",
				""),
			step(2,
				"Or register manually: `sctool cluster add --host <scylla-ip> "+
					"--port <agent-https-port> --name globular-internal "+
					"--auth-token <from-scylla-manager-agent.yaml>`",
				""),
		},
	}
}

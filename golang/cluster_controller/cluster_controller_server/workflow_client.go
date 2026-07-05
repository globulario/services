package main

// workflow_client.go — health-aware, failover client accessor for the workflow
// service. Replaces the previous boot-pinned single client, which never
// re-resolved: if the one dialed instance died or was slow, the controller had
// no way to reach another. That contributed to "workflow down => cluster
// blocked". Here the controller re-resolves per call and fails over to a healthy
// instance.
//
// Direct-dial ONLY — never the Envoy mesh. The control plane must not depend on
// the data-plane mesh it manages
// (invariant.control_plane_must_not_depend_on_the_data_plane_mesh_it_mana). This
// is the same deliberate exception to globular.pattern.grpc_client_standard that
// the workflow recorder and actor-callback paths already make: the standard
// client routes through Envoy (:443), which would couple control-plane dispatch
// to the mesh and strip the mTLS+token auth the workflow service requires.

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// getWorkflowClient returns a WorkflowService client aimed at a healthy workflow
// instance, or nil if none is reachable (callers treat nil like the old
// "workflowClient == nil"). It re-resolves on every call (fresh evidence —
// intent:health.requires_fresh_evidence) and fails over: a RUNNING instance is
// preferred, the LOCAL node first, and a cached connection stuck in
// TransientFailure/Shutdown is skipped so a dead instance is bypassed even
// before etcd's registered State catches up. Connections are cached per address.
func (srv *server) getWorkflowClient() workflowpb.WorkflowServiceClient {
	cands := srv.resolveWorkflowCandidates()
	if len(cands) == 0 {
		return nil
	}
	clusterID := strings.TrimSpace(srv.cfg.ClusterDomain)
	if clusterID == "" {
		clusterID = "globular.internal"
	}

	srv.wfMu.Lock()
	defer srv.wfMu.Unlock()
	if srv.wfConns == nil {
		srv.wfConns = make(map[string]*grpc.ClientConn)
	}
	for _, addr := range cands {
		conn := srv.wfConns[addr]
		if conn != nil {
			switch conn.GetState() {
			case connectivity.Shutdown:
				conn.Close()
				delete(srv.wfConns, addr)
				conn = nil
			case connectivity.TransientFailure:
				// Unhealthy — skip to the next candidate and let gRPC keep
				// retrying this one in the background; it is reused once Ready.
				continue
			}
		}
		if conn == nil {
			dt := config.ResolveDialTarget(addr)
			c, err := grpc.NewClient(dt.Address,
				grpc.WithTransportCredentials(buildControllerClientTLSCreds(dt.ServerName)),
				grpc.WithUnaryInterceptor(controllerTokenInterceptor(clusterID)))
			if err != nil {
				log.Printf("cluster-controller: workflow client dial %s failed: %v", dt.Address, err)
				continue
			}
			srv.wfConns[addr] = c
			conn = c
		}
		return workflowpb.NewWorkflowServiceClient(conn)
	}
	return nil
}

// resolveWorkflowCandidates returns direct (mesh-bypassing) addresses of workflow
// instances ordered by preference: running+local, running+remote, then the rest
// as a last resort. Loopback and portless registrations are dropped — a loopback
// is never a valid cluster endpoint.
func (srv *server) resolveWorkflowCandidates() []string {
	svcs, err := config.GetServicesConfigurationsByName("workflow.WorkflowService")
	if err != nil || len(svcs) == 0 {
		return nil
	}
	localIP := config.GetRoutableIPv4()
	var cands []wfCandidate
	for _, s := range svcs {
		host, _ := s["Address"].(string)
		if h, _, e := net.SplitHostPort(host); e == nil {
			host = h
		}
		host = strings.TrimSpace(host)
		if host == "" || host == "localhost" || host == "127.0.0.1" {
			continue
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			continue
		}
		var port int
		switch p := s["Port"].(type) {
		case float64:
			port = int(p)
		case int:
			port = p
		}
		if port == 0 {
			continue
		}
		cands = append(cands, wfCandidate{
			addr:    net.JoinHostPort(host, strconv.Itoa(port)),
			local:   host == localIP,
			running: strings.EqualFold(fmt.Sprint(s["State"]), "running"),
		})
	}
	return orderWorkflowCandidates(cands)
}

// wfCandidate is a parsed workflow-instance endpoint with its locality/health.
type wfCandidate struct {
	addr           string
	local, running bool
}

// orderWorkflowCandidates returns candidate addresses in failover preference
// order — running+local, running+remote, then non-running (local before remote)
// as a last resort — deduplicated. Pure, so the preference policy is testable.
func orderWorkflowCandidates(cands []wfCandidate) []string {
	seen := make(map[string]bool, len(cands))
	var out []string
	for _, want := range []struct{ running, local bool }{
		{true, true}, {true, false}, {false, true}, {false, false},
	} {
		for _, c := range cands {
			if c.running == want.running && c.local == want.local && !seen[c.addr] {
				seen[c.addr] = true
				out = append(out, c.addr)
			}
		}
	}
	return out
}

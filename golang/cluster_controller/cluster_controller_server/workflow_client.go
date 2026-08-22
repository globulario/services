package main

// workflow_client.go — health-aware, failing-over accessor for the workflow
// service.
//
// It replaces a boot-pinned single client. The old path resolved ONE workflow
// address at startup, dialled it, assigned srv.workflowClient, and returned. It
// never re-resolved. Every caller guarded with `if srv.workflowClient == nil`,
// which stays false once the field is set — so after that one instance died the
// guard kept passing and the controller kept dispatching into a dead
// connection. A nil-check that only proves "we dialled something once, once" is
// not a liveness check; it is the shape of a fail-open. That contributed
// directly to "workflow down => cluster blocked".
//
// The accessor re-resolves on every call (fresh evidence —
// intent:health.requires_fresh_evidence) and fails over: a RUNNING instance is
// preferred, the LOCAL node first, and a cached connection sitting in
// TransientFailure/Shutdown is skipped so a dead instance is bypassed even
// before etcd's registered State catches up. Connections are cached per address
// so failover does not mean redialling on every call.
//
// Direct-dial ONLY — never the Envoy mesh. The control plane must not depend on
// the data-plane mesh it manages
// (invariant.control_plane_must_not_depend_on_the_data_plane_mesh_it_mana).
// This is the same deliberate exception to globular.pattern.grpc_client_standard
// that the workflow recorder and actor-callback paths already make: the
// standard client routes through Envoy (:443), which would couple control-plane
// dispatch to the mesh and strip the mTLS+token auth the workflow service
// requires.

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
// instance, or nil if none is reachable. Callers treat nil exactly as they
// treated the old "workflowClient == nil" — the difference is that nil now means
// "no instance is reachable right now", re-evaluated per call, rather than "we
// never managed to dial at boot".
func (srv *server) getWorkflowClient() workflowpb.WorkflowServiceClient {
	// Explicit override wins. Tests inject a fake here; production leaves it nil
	// so resolution below is always the live path.
	if srv.workflowClient != nil {
		return srv.workflowClient
	}

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
				_ = conn.Close()
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
// instances ordered by preference. Loopback and portless registrations are
// dropped — a loopback is never a valid cluster endpoint (HARD RULE 3).
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
		// config.IsLoopbackEndpoint is the single loopback predicate: it covers
		// "localhost", the whole 127.0.0.0/8 range and ::1, and splits host:port
		// itself. Hand-rolled comparisons drift — this one already missed ::1.
		if host == "" || config.IsLoopbackEndpoint(host) {
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
// as a last resort — deduplicated. Pure, so the preference policy is testable
// without a cluster.
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

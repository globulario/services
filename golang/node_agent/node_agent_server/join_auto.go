package main

import (
	"context"
	"log"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// autoInitiateJoin submits a join request to the controller when a join token
// is present in state but no request has been submitted yet.
//
// v2 path (preferred): when state.JoinID is non-empty, the installer already
// called /join/authorize and obtained a signed JoinPlan. The node-agent
// validates the stored plan, records the join_id as request_id, and polls
// GetJoinRequestStatus without consuming the join token again.
//
// v1 legacy path: when state.JoinID is empty, the node-agent calls RequestJoin
// directly (old behavior). Explicitly logged as "v1 legacy path".
//
// Called once at startup (as a goroutine) when:
//   - joinToken != "" AND joinRequestID == "" (v1), OR
//   - state.JoinID != "" AND joinRequestID == "" (v2)
//
// Stops when ctx is cancelled or the request is accepted/permanently rejected.
func (srv *NodeAgentServer) autoInitiateJoin(ctx context.Context) {
	// ── v2 path: installer pre-obtained a signed JoinPlan ────────────────────
	srv.stateMu.Lock()
	joinID := srv.state.JoinID
	joinPlanJSON := srv.state.JoinPlanJSON
	srv.stateMu.Unlock()

	if joinID != "" {
		log.Printf("join: v2 path — join_id=%s from installer (JoinPlan pre-authorized)", joinID)

		// Validate the stored plan when the node-agent has plan bytes.
		// Signature verification uses the keystore (production) or a test hook.
		// We do not block startup on missing key material — the plan may have
		// been validated by the installer already; log and proceed.
		if len(joinPlanJSON) > 0 {
			_, err := validateNodeJoinPlan(joinPlanJSON, NodeJoinPlanParams{
				SkipSignatureVerification: !joinPlanKeystoreReady(),
			})
			if err != nil {
				log.Printf("join: v2 path: join plan validation failed: %v — blocking join", err)
				return
			}
			log.Printf("join: v2 path: join plan accepted: join_id=%s", joinID)
		}

		srv.stateMu.Lock()
		srv.joinRequestID = joinID
		srv.state.RequestID = joinID
		srv.stateMu.Unlock()
		if err := srv.saveState(); err != nil {
			log.Printf("join: v2 path: warn: save state: %v", err)
		}

		if err := srv.ensureControllerClient(ctx); err != nil {
			log.Printf("join: v2 path: controller unreachable — will retry on heartbeat: %v", err)
		}
		srv.startJoinApprovalWatcher(ctx, joinID)
		return
	}

	// ── v1 legacy path: call RequestJoin directly ─────────────────────────────
	token := srv.joinToken
	if token == "" {
		return
	}
	log.Printf("join: v1 legacy path — controller=%s (no pre-obtained JoinPlan)", srv.controllerEndpoint)

	backoff := 5 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		srv.stateMu.Lock()
		alreadyRequested := srv.joinRequestID != "" || srv.joinToken == ""
		srv.stateMu.Unlock()
		if satisfied, nodeID := srv.legacyJoinSatisfiedByStatusReport(); satisfied {
			log.Printf("join: v1 legacy path skipped — node already registered via ReportNodeStatus (node_id=%s)", nodeID)
			return
		}
		if alreadyRequested {
			log.Printf("join: v1 legacy path: request already in flight or token cleared — stopping")
			return
		}

		if err := srv.ensureControllerClient(ctx); err != nil {
			log.Printf("join: v1 legacy path: controller unreachable: %v — retry in %s", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				if backoff < maxBackoff {
					backoff *= 2
				}
			}
			continue
		}

		resp, err := srv.controllerClient.RequestJoin(ctx, &cluster_controllerpb.RequestJoinRequest{
			JoinToken:    token,
			Identity:     srv.buildNodeIdentity(),
			Labels:       srv.joinRequestLabels(),
			Capabilities: buildNodeCapabilities(),
		})
		if err != nil {
			if st, ok := status.FromError(err); ok {
				switch st.Code() {
				case codes.PermissionDenied, codes.NotFound, codes.InvalidArgument:
					log.Printf("join: v1 legacy path: non-retriable rejection from controller: %v", err)
					return
				}
			}
			log.Printf("join: v1 legacy path: RequestJoin failed: %v — retry in %s", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				if backoff < maxBackoff {
					backoff *= 2
				}
			}
			continue
		}

		requestID := resp.GetRequestId()
		if requestID == "" {
			log.Printf("join: v1 legacy path: controller returned empty request_id — retry in %s", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		srv.stateMu.Lock()
		srv.joinRequestID = requestID
		srv.state.RequestID = requestID
		srv.stateMu.Unlock()
		if err := srv.saveState(); err != nil {
			log.Printf("join: v1 legacy path: warn: persist join request state: %v", err)
		}
		log.Printf("join: v1 legacy path: join request submitted (id=%s status=%s) — polling for approval",
			requestID, resp.GetStatus())
		srv.startJoinApprovalWatcher(ctx, requestID)
		return
	}
}

// joinPlanKeystoreReady reports whether the security keystore is wired up for
// join plan verification. When false, signature verification is skipped and
// the plan is accepted on structural validity alone — expected during early
// bootstrapping before the security package is initialized.
func joinPlanKeystoreReady() bool {
	return security.GetPeerPublicKey != nil
}

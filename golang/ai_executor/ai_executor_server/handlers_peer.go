package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// Ping returns this executor's identity and capabilities.
func (srv *server) Ping(ctx context.Context, req *ai_executorpb.PeerPingRequest) (*ai_executorpb.PeerPingResponse, error) {
	hostname, _ := os.Hostname()
	// ai_available means the autonomous diagnosis backend is actually usable —
	// NOT "a claude binary exists on disk". The latter made this field report
	// true on a cluster with no credentials, where every diagnosis was the
	// deterministic fallback. See backendReadiness / aiReady.
	aiAvailable := srv.diagnoser.aiReady()

	srv.statsMu.Lock()
	processed := srv.stats.IncidentsProcessed
	srv.statsMu.Unlock()

	var profiles []string
	if srv.peers != nil {
		profiles = srv.peers.localProfiles
	}

	return &ai_executorpb.PeerPingResponse{
		NodeId:             srv.GetId(),
		Hostname:           hostname,
		AiAvailable:        aiAvailable,
		Profiles:           profiles,
		UptimeSeconds:      int64(time.Since(srv.startedAt).Seconds()),
		IncidentsProcessed: processed,
	}, nil
}

// ShareObservation receives an observation from a peer and checks local state.
func (srv *server) ShareObservation(ctx context.Context, req *ai_executorpb.PeerObservationRequest) (*ai_executorpb.PeerObservationResponse, error) {
	nodeID := srv.GetId()

	// Use the diagnoser's AI (if available) to evaluate the observation locally.
	// Otherwise, do a simple deterministic check.
	confirmed := false
	localEvidence := ""
	var confidence float32

	switch req.Category {
	case "service_crash", "service_failed":
		// Check cluster health for the named service — os/exec is not allowed
		// outside node_agent; routing through cluster controller is the correct path.
		unit := req.Subject
		if unit != "" {
			healthy, reason := checkServiceHealth(ctx, unit)
			if !healthy {
				confirmed = true
				localEvidence = reason
				confidence = 0.7 // lower than os/exec: cluster-level, not node-local
			} else {
				localEvidence = reason
				confidence = 0.8
			}
		}
	case "high_latency", "disk_full", "memory_pressure":
		// These are typically node-local — unlikely to confirm on peer.
		localEvidence = "not observed locally"
		confidence = 0.5
	default:
		// Ask AI if available.
		if srv.diagnoser != nil {
			prompt := "A peer node (" + req.SenderHostname + ") reports: " +
				req.Category + " on " + req.Subject + ". " +
				"Details: " + req.Detail + ". " +
				"Do you see the same issue on this node? Check local state and respond with a brief assessment."
			response, err := srv.diagnoser.sendPrompt(ctx, prompt)
			if err == nil {
				localEvidence = response
				confidence = 0.7
			}
		}
	}

	return &ai_executorpb.PeerObservationResponse{
		NodeId:        nodeID,
		Confirmed:     confirmed,
		LocalEvidence: localEvidence,
		Confidence:    confidence,
	}, nil
}

// ProposeAction receives a proposed action from a peer and votes.
func (srv *server) ProposeAction(ctx context.Context, req *ai_executorpb.PeerProposalRequest) (*ai_executorpb.PeerProposalResponse, error) {
	nodeID := srv.GetId()

	// Default: abstain if no AI.
	vote := ai_executorpb.PeerVote_VOTE_ABSTAIN
	reason := "no AI backend available for evaluation"

	if srv.diagnoser != nil {
		// Vote with AI only when the autonomous backend is truly usable; the
		// claude CLI binary existing is not a usable autonomous backend.
		aiAvailable := srv.diagnoser.aiReady()

		if aiAvailable {
			// Ask AI to evaluate the proposal.
			prompt := "A peer executor proposes the following action:\n" +
				"- Action: " + req.ProposedAction.String() + "\n" +
				"- Target: " + req.Target + "\n" +
				"- Rationale: " + req.Rationale + "\n" +
				"- Tier: " + string(rune('0'+req.Tier)) + "\n\n"

			if req.Diagnosis != nil {
				prompt += "Diagnosis:\n" +
					"- Root cause: " + req.Diagnosis.RootCause + "\n" +
					"- Confidence: " + formatFloat(req.Diagnosis.Confidence) + "\n" +
					"- Summary: " + req.Diagnosis.Summary + "\n\n"
			}

			prompt += "Should this action be executed? Respond with exactly one word: APPROVE, REJECT, or ESCALATE. " +
				"Then on the next line, explain why in one sentence."

			response, err := srv.diagnoser.sendPrompt(ctx, prompt)
			if err == nil {
				vote, reason = parseVoteResponse(response)
			} else {
				reason = "AI evaluation failed: " + err.Error()
			}
		}
	}

	// Safety override: always escalate high-risk actions regardless of AI opinion.
	if req.Tier >= 2 && vote == ai_executorpb.PeerVote_VOTE_APPROVE {
		vote = ai_executorpb.PeerVote_VOTE_ESCALATE
		reason = "tier 2+ actions require human approval (safety policy)"
		go emitBehavioralSelfOperation(ctx, newSafetyHookSelfOperation(req, reason))
	} else if vote == ai_executorpb.PeerVote_VOTE_REJECT || vote == ai_executorpb.PeerVote_VOTE_ESCALATE {
		go emitBehavioralSelfOperation(ctx, newClassifierDenialSelfOperation(req, vote, reason))
	}

	return &ai_executorpb.PeerProposalResponse{
		NodeId: nodeID,
		Vote:   vote,
		Reason: reason,
	}, nil
}

// NotifyActionTaken acknowledges an action taken by a peer.
func (srv *server) NotifyActionTaken(ctx context.Context, req *ai_executorpb.PeerActionNotification) (*ai_executorpb.PeerActionAck, error) {
	logger.Info("peer-action-notify",
		"from", req.SenderNodeId,
		"incident", req.IncidentId,
		"action", req.Action.String(),
		"target", req.Target,
		"status", req.Status.String())

	return &ai_executorpb.PeerActionAck{
		NodeId:       srv.GetId(),
		Acknowledged: true,
	}, nil
}

// --- Helpers ---

// checkServiceHealth queries the cluster controller to see if any node reports
// the named service as degraded or missing. This replaces a direct os/exec
// systemctl call (which violates the boundary: only node_agent may exec systemctl).
// The check is coarser (cluster-wide, not local) but architecturally correct.
func checkServiceHealth(ctx context.Context, serviceName string) (healthy bool, reason string) {
	addr := config.ResolveServiceAddr("clustercontroller.ClusterControllerService", "")
	if addr == "" {
		return false, "cluster controller not found — cannot check service health"
	}

	baseOpts, err := globular.InternalDialOptions()
	if err != nil {
		return false, "internal TLS unavailable: " + err.Error()
	}
	//nolint:staticcheck // grpc.Dial / grpc.WithTimeout not yet migrated to grpc.NewClient
	opts := append(baseOpts, grpc.WithTimeout(2*time.Second))
	//nolint:staticcheck
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return false, "cannot dial cluster controller: " + err.Error()
	}
	defer cc.Close()

	health, err := cluster_controllerpb.NewClusterControllerServiceClient(cc).
		GetClusterHealth(ctx, &cluster_controllerpb.GetClusterHealthRequest{})
	if err != nil {
		return false, "health check RPC failed: " + err.Error()
	}

	for _, nh := range health.GetNodeHealth() {
		if nh.GetStatus() != "healthy" && nh.GetStatus() != "ready" {
			// A degraded node may be running the service; treat as unconfirmed-unhealthy.
			return false, fmt.Sprintf("node %s is %s (service %s may be affected)", nh.GetNodeId(), nh.GetStatus(), serviceName)
		}
	}
	return true, fmt.Sprintf("all nodes healthy — service %s likely running", serviceName)
}

func formatFloat(f float32) string {
	return fmt.Sprintf("%.2f", f)
}

func parseVoteResponse(response string) (ai_executorpb.PeerVote, string) {
	lines := strings.Split(strings.TrimSpace(response), "\n")
	if len(lines) == 0 {
		return ai_executorpb.PeerVote_VOTE_ABSTAIN, "empty response"
	}

	first := strings.ToUpper(strings.TrimSpace(lines[0]))
	reason := ""
	if len(lines) > 1 {
		reason = strings.TrimSpace(strings.Join(lines[1:], " "))
	}

	switch {
	case strings.Contains(first, "APPROVE"):
		return ai_executorpb.PeerVote_VOTE_APPROVE, reason
	case strings.Contains(first, "REJECT"):
		return ai_executorpb.PeerVote_VOTE_REJECT, reason
	case strings.Contains(first, "ESCALATE"):
		return ai_executorpb.PeerVote_VOTE_ESCALATE, reason
	default:
		return ai_executorpb.PeerVote_VOTE_ABSTAIN, "could not parse vote: " + first
	}
}

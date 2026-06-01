package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

var execCommand = exec.Command

// Ping returns this executor's identity and capabilities.
func (srv *server) Ping(ctx context.Context, req *ai_executorpb.PeerPingRequest) (*ai_executorpb.PeerPingResponse, error) {
	hostname, _ := os.Hostname()
	aiAvailable := srv.diagnoser != nil &&
		((srv.diagnoser.anthropic != nil && srv.diagnoser.anthropic.isAvailable()) ||
			(srv.diagnoser.claude != nil && srv.diagnoser.claude.isAvailable()))

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
		// Check if the same service is unhealthy on this node.
		unit := req.Subject
		if unit != "" {
			healthy, reason := checkLocalUnit(unit)
			if !healthy {
				confirmed = true
				localEvidence = reason
				confidence = 0.9
			} else {
				localEvidence = "service healthy on this node"
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
		aiAvailable := (srv.diagnoser.anthropic != nil && srv.diagnoser.anthropic.isAvailable()) ||
			(srv.diagnoser.claude != nil && srv.diagnoser.claude.isAvailable())

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

func checkLocalUnit(unit string) (bool, string) {
	// Quick systemd check — reuse the same pattern as event publisher.
	// Returns (healthy, reason).
	cmd := execCommand("systemctl", "is-active", unit)
	out, err := cmd.Output()
	if err != nil {
		return false, "unit " + unit + " is not active"
	}
	state := string(out)
	if state == "active\n" || state == "active" {
		return true, ""
	}
	return false, "unit state: " + state
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

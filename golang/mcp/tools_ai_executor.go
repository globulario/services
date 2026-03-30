package main

import (
	"context"
	"fmt"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

func registerAIExecutorTools(s *server) {

	// Helper: get an ai-executor client via gateway.
	getClient := func(ctx context.Context) (ai_executorpb.AiExecutorServiceClient, error) {
		conn, err := s.clients.get(ctx, gatewayEndpoint())
		if err != nil {
			return nil, err
		}
		return ai_executorpb.NewAiExecutorServiceClient(conn), nil
	}

	// ── ai_executor_status ───────────────────────────────────────────
	s.register(toolDef{
		Name: "ai_executor_status",
		Description: "Get the AI executor's operational status and check if the AI backend " +
			"(Anthropic API / Claude) is active. Shows peer count for multi-node consensus.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("ai_executor_status: %w", err)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		status, err := client.GetStatus(callCtx, &ai_executorpb.GetStatusRequest{})
		if err != nil {
			return nil, fmt.Errorf("ai_executor_status: %w", err)
		}

		ping, _ := client.Ping(callCtx, &ai_executorpb.PeerPingRequest{
			SenderNodeId:   "mcp",
			SenderHostname: "mcp",
		})

		result := map[string]interface{}{
			"incidents_processed": status.IncidentsProcessed,
			"diagnoses_completed": status.DiagnosesCompleted,
			"actions_executed":    status.ActionsExecuted,
			"actions_failed":      status.ActionsFailed,
			"uptime_seconds":      status.UptimeSeconds,
		}
		if ping != nil {
			result["hostname"] = ping.Hostname
			result["ai_available"] = ping.AiAvailable
			result["profiles"] = ping.Profiles
		}
		return result, nil
	})

	// ── ai_executor_list_peers ───────────────────────────────────────
	s.register(toolDef{
		Name: "ai_executor_list_peers",
		Description: "List all AI executor instances across the cluster. Pings each one " +
			"to check AI availability, uptime, and profiles. Use this to verify the " +
			"distributed AI consensus network.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		services, err := config.GetServicesConfigurations()
		if err != nil {
			return nil, fmt.Errorf("ai_executor_list_peers: %w", err)
		}

		peers := make([]map[string]interface{}, 0)
		for _, svc := range services {
			name := Utility.ToString(svc["Name"])
			if name != "ai_executor.AiExecutorService" {
				continue
			}
			addr := Utility.ToString(svc["Address"])
			port := Utility.ToInt(svc["Port"])
			if addr == "" || port == 0 {
				continue
			}
			endpoint := fmt.Sprintf("%s:%d", addr, port)

			conn, err := s.clients.get(ctx, endpoint)
			if err != nil {
				peers = append(peers, map[string]interface{}{
					"endpoint": endpoint, "status": "unreachable",
				})
				continue
			}

			client := ai_executorpb.NewAiExecutorServiceClient(conn)
			callCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
			resp, err := client.Ping(callCtx, &ai_executorpb.PeerPingRequest{
				SenderNodeId: "mcp", SenderHostname: "mcp",
			})
			cancel()

			if err != nil {
				peers = append(peers, map[string]interface{}{
					"endpoint": endpoint, "status": "error", "error": err.Error(),
				})
				continue
			}

			peers = append(peers, map[string]interface{}{
				"endpoint":            endpoint,
				"node_id":             resp.NodeId,
				"hostname":            resp.Hostname,
				"ai_available":        resp.AiAvailable,
				"profiles":            resp.Profiles,
				"uptime_seconds":      resp.UptimeSeconds,
				"incidents_processed": resp.IncidentsProcessed,
				"status":              "healthy",
			})
		}

		return map[string]interface{}{"total": len(peers), "peers": peers}, nil
	})

	// ── ai_executor_send_prompt ──────────────────────────────────────
	s.register(toolDef{
		Name: "ai_executor_send_prompt",
		Description: "Send a prompt to the cluster's AI executor. The executor routes it " +
			"through the Anthropic API (Max subscription) or Claude CLI. " +
			"Use this to talk to the AI that lives ON the cluster — a separate Claude " +
			"instance from this conversation. Supports multi-turn conversations via conversation_id " +
			"and routing to specific nodes via target_node.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"prompt":          {Type: "string", Description: "The prompt to send to the cluster AI"},
				"conversation_id": {Type: "string", Description: "Continue an existing conversation (empty = new)"},
				"target_node":     {Type: "string", Description: "Route to a specific peer node by hostname"},
			},
			Required: []string{"prompt"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		prompt := strArg(args, "prompt")
		if prompt == "" {
			return nil, fmt.Errorf("prompt is required")
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("ai_executor_send_prompt: %w", err)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 120*time.Second)
		defer cancel()

		stream, err := client.SendPrompt(callCtx, &ai_executorpb.SendPromptRequest{
			Prompt:         prompt,
			ConversationId: strArg(args, "conversation_id"),
			TargetNode:     strArg(args, "target_node"),
			Metadata:       map[string]string{"source": "mcp"},
		})
		if err != nil {
			return nil, fmt.Errorf("ai_executor_send_prompt: %w", err)
		}

		// Collect streaming response.
		var fullText, convID, respondingNode, questionForHuman string
		var inputTokens, outputTokens int32
		var needsReply bool

		for {
			resp, err := stream.Recv()
			if err != nil {
				break
			}
			if resp.ConversationId != "" {
				convID = resp.ConversationId
			}
			if resp.FullText != "" {
				fullText = resp.FullText
			}
			if resp.TextChunk != "" {
				fullText += resp.TextChunk
			}
			if resp.RespondingNode != "" {
				respondingNode = resp.RespondingNode
			}
			if resp.Done {
				inputTokens = resp.InputTokens
				outputTokens = resp.OutputTokens
				needsReply = resp.NeedsHumanReply
				questionForHuman = resp.QuestionForHuman
				break
			}
		}

		result := map[string]interface{}{
			"response":        fullText,
			"conversation_id": convID,
			"responding_node": respondingNode,
			"input_tokens":    inputTokens,
			"output_tokens":   outputTokens,
		}
		if needsReply {
			result["needs_human_reply"] = true
			result["question_for_human"] = questionForHuman
		}
		return result, nil
	})

	// ── ai_executor_list_jobs ────────────────────────────────────────
	s.register(toolDef{
		Name: "ai_executor_list_jobs",
		Description: "List recent AI executor jobs — incident diagnoses and remediation actions.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"limit": {Type: "number", Description: "Max results (default 10)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("ai_executor_list_jobs: %w", err)
		}
		limit := int32(10)
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int32(l)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.ListJobs(callCtx, &ai_executorpb.ListJobsRequest{Limit: limit})
		if err != nil {
			return nil, fmt.Errorf("ai_executor_list_jobs: %w", err)
		}

		jobs := make([]map[string]interface{}, 0)
		for _, j := range resp.GetJobs() {
			job := map[string]interface{}{
				"incident_id": j.IncidentId,
				"state":       j.State.String(),
				"tier":        j.Tier,
				"action":      j.ActionType.String(),
				"target":      j.ActionTarget,
			}
			if j.Diagnosis != nil {
				job["root_cause"] = j.Diagnosis.RootCause
				job["summary"] = j.Diagnosis.Summary
			}
			if j.Error != "" {
				job["error"] = j.Error
			}
			jobs = append(jobs, job)
		}
		return map[string]interface{}{"total": len(jobs), "jobs": jobs}, nil
	})

	// ── ai_executor_share_observation ────────────────────────────────
	s.register(toolDef{
		Name: "ai_executor_share_observation",
		Description: "Share an observation with the cluster's AI executor for peer confirmation. " +
			"The executor checks its local state and asks peer nodes if they see the same issue.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"category": {Type: "string", Description: "Category: service_crash, high_latency, disk_full, memory_pressure, etc."},
				"subject":  {Type: "string", Description: "What's affected (service name, unit, path)"},
				"detail":   {Type: "string", Description: "Description of the observation"},
			},
			Required: []string{"category", "subject", "detail"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("ai_executor_share_observation: %w", err)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.ShareObservation(callCtx, &ai_executorpb.PeerObservationRequest{
			ObservationId:  fmt.Sprintf("mcp-obs-%d", time.Now().UnixMilli()),
			SenderNodeId:   "mcp",
			SenderHostname: "mcp",
			Category:       strArg(args, "category"),
			Subject:        strArg(args, "subject"),
			Detail:         strArg(args, "detail"),
			ObservedAtMs:   time.Now().UnixMilli(),
		})
		if err != nil {
			return nil, fmt.Errorf("ai_executor_share_observation: %w", err)
		}

		return map[string]interface{}{
			"node_id":        resp.NodeId,
			"confirmed":      resp.Confirmed,
			"local_evidence": resp.LocalEvidence,
			"confidence":     resp.Confidence,
		}, nil
	})
}

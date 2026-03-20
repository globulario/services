package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
)

// learningStore records routing decisions in ai_memory and queries historical
// patterns to improve future scoring.
type learningStore struct {
	memoryAddr   string
	memoryConn   *grpc.ClientConn
	memoryClient ai_memorypb.AiMemoryServiceClient
}

func newLearningStore() *learningStore {
	return &learningStore{}
}

// connect establishes a connection to ai_memory service.
func (ls *learningStore) connect() error {
	if ls.memoryClient != nil {
		return nil
	}

	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		return fmt.Errorf("ai_memory service not found")
	}
	ls.memoryAddr = addr

	cc, err := grpc.Dial(addr,
		globular.InternalDialOption(),
		grpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		return fmt.Errorf("connect to ai_memory %s: %w", addr, err)
	}
	ls.memoryConn = cc
	ls.memoryClient = ai_memorypb.NewAiMemoryServiceClient(cc)
	logger.Info("learning: connected to ai_memory", "addr", addr)
	return nil
}

// recordDecision stores a routing decision in ai_memory for future learning.
func (ls *learningStore) recordDecision(ctx context.Context, decision *routingDecisionRecord) {
	if ls.memoryClient == nil {
		if err := ls.connect(); err != nil {
			return
		}
	}

	data, err := json.Marshal(decision)
	if err != nil {
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err = ls.memoryClient.Store(callCtx, &ai_memorypb.StoreRqst{
		Memory: &ai_memorypb.Memory{
			Project: "globular-services",
			Type:    ai_memorypb.MemoryType_DECISION,
			Title:   fmt.Sprintf("routing: %s (%d endpoints changed)", decision.Summary, decision.EndpointsChanged),
			Content: string(data),
			Tags:    []string{"routing", "ai-router", "decision"},
			Metadata: map[string]string{
				"confidence": fmt.Sprintf("%.2f", decision.Confidence),
				"cycle":      fmt.Sprintf("%d", decision.Cycle),
				"mode":       decision.Mode,
			},
		},
	})
	if err != nil {
		logger.Debug("learning: failed to store decision", "err", err)
	}
}

// queryBaseline queries ai_memory for historical routing patterns.
// Returns the average score for a service over recent decisions.
func (ls *learningStore) queryBaseline(ctx context.Context, service string) float64 {
	if ls.memoryClient == nil {
		if err := ls.connect(); err != nil {
			return 0
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	resp, err := ls.memoryClient.Query(callCtx, &ai_memorypb.QueryRqst{
		Project:    "globular-services",
		Type:       ai_memorypb.MemoryType_DECISION,
		Tags:       []string{"routing"},
		TextSearch: service,
		Limit:      10,
	})
	if err != nil || resp == nil || len(resp.Memories) == 0 {
		return 0
	}

	var totalScore float64
	var count int
	for _, mem := range resp.Memories {
		var record routingDecisionRecord
		if err := json.Unmarshal([]byte(mem.Content), &record); err != nil {
			continue
		}
		for _, ep := range record.Endpoints {
			if ep.Service == service {
				totalScore += ep.Score
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}
	return totalScore / float64(count)
}

// routingDecisionRecord is the structured data stored in ai_memory.
type routingDecisionRecord struct {
	Cycle            uint64                   `json:"cycle"`
	Mode             string                   `json:"mode"`
	Confidence       float64                  `json:"confidence"`
	EndpointsChanged int                      `json:"endpoints_changed"`
	SafetyClamps     int                      `json:"safety_clamps"`
	ActiveDrains     int                      `json:"active_drains"`
	Summary          string                   `json:"summary"`
	Endpoints        []endpointDecisionRecord `json:"endpoints,omitempty"`
	CPU              float64                  `json:"cpu"`
	Memory           float64                  `json:"memory"`
	Timestamp        time.Time                `json:"timestamp"`
}

type endpointDecisionRecord struct {
	Service  string   `json:"service"`
	Instance string   `json:"instance"`
	Score    float64  `json:"score"`
	Weight   uint32   `json:"weight"`
	Reasons  []string `json:"reasons,omitempty"`
}

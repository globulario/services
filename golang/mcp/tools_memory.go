package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
)

// memoryEndpoint returns the AI memory service endpoint.
func memoryEndpoint() string {
	return gatewayEndpoint()
}

// parseMemoryType converts a string to the proto enum.
func parseMemoryType(s string) ai_memorypb.MemoryType {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "FEEDBACK":
		return ai_memorypb.MemoryType_FEEDBACK
	case "ARCHITECTURE":
		return ai_memorypb.MemoryType_ARCHITECTURE
	case "DECISION":
		return ai_memorypb.MemoryType_DECISION
	case "DEBUG":
		return ai_memorypb.MemoryType_DEBUG
	case "SESSION":
		return ai_memorypb.MemoryType_SESSION
	case "USER":
		return ai_memorypb.MemoryType_USER
	case "PROJECT":
		return ai_memorypb.MemoryType_PROJECT
	case "REFERENCE":
		return ai_memorypb.MemoryType_REFERENCE
	case "SCRATCH":
		return ai_memorypb.MemoryType_SCRATCH
	case "SKILL":
		return ai_memorypb.MemoryType_SKILL
	default:
		return ai_memorypb.MemoryType_MEMORY_UNSPECIFIED
	}
}

// memoryTypeString converts the proto enum to a human-readable string.
func memoryTypeString(t ai_memorypb.MemoryType) string {
	switch t {
	case ai_memorypb.MemoryType_FEEDBACK:
		return "feedback"
	case ai_memorypb.MemoryType_ARCHITECTURE:
		return "architecture"
	case ai_memorypb.MemoryType_DECISION:
		return "decision"
	case ai_memorypb.MemoryType_DEBUG:
		return "debug"
	case ai_memorypb.MemoryType_SESSION:
		return "session"
	case ai_memorypb.MemoryType_USER:
		return "user"
	case ai_memorypb.MemoryType_PROJECT:
		return "project"
	case ai_memorypb.MemoryType_REFERENCE:
		return "reference"
	case ai_memorypb.MemoryType_SCRATCH:
		return "scratch"
	case ai_memorypb.MemoryType_SKILL:
		return "skill"
	default:
		return "unspecified"
	}
}

func registerMemoryTools(s *server) {

	// ── memory_store ────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "memory_store",
		Description: "Store a memory entry in the AI knowledge base. Use this to persist " +
			"knowledge, decisions, debugging insights, user preferences, or session " +
			"context that should survive across conversations. Memories are cluster-scoped " +
			"and searchable by tags and type.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project": {Type: "string", Description: "Project identifier, e.g. 'globular-services'"},
				"type": {Type: "string", Description: "Memory type: feedback, architecture, decision, debug, session, user, project, reference, scratch",
					Enum: []string{"feedback", "architecture", "decision", "debug", "session", "user", "project", "reference", "scratch"}},
				"tags":            {Type: "string", Description: "Comma-separated tags for searchability, e.g. 'dns,badgerdb,corruption'"},
				"title":           {Type: "string", Description: "Short summary (one line) for listing"},
				"content":         {Type: "string", Description: "Full memory content — structured text, markdown, etc."},
				"ttl_seconds":     {Type: "number", Description: "Time-to-live in seconds. 0 = permanent. Use for scratch/ephemeral memories."},
				"conversation_id": {Type: "string", Description: "Optional: link to the originating conversation"},
				"metadata":        {Type: "string", Description: "JSON object of key-value pairs for flexible attributes, e.g. '{\"root_cause\":\"unclean-shutdown\",\"confidence\":\"high\"}'"},
				"related_ids":     {Type: "string", Description: "Comma-separated memory IDs that this memory is related to"},
			},
			Required: []string{"project", "type", "title", "content"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		memory := &ai_memorypb.Memory{
			Project:   strArg(args, "project"),
			Type:      parseMemoryType(strArg(args, "type")),
			Title:     strArg(args, "title"),
			Content:   strArg(args, "content"),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
			AgentId:   "claude-mcp",
		}

		if tags := strArg(args, "tags"); tags != "" {
			for _, t := range strings.Split(tags, ",") {
				if t = strings.TrimSpace(t); t != "" {
					memory.Tags = append(memory.Tags, t)
				}
			}
		}
		if ttl, ok := args["ttl_seconds"].(float64); ok {
			memory.TtlSeconds = int32(ttl)
		}
		if cid := strArg(args, "conversation_id"); cid != "" {
			memory.ConversationId = cid
		}
		if md := strArg(args, "metadata"); md != "" {
			parsed := make(map[string]string)
			if err := json.Unmarshal([]byte(md), &parsed); err == nil {
				memory.Metadata = parsed
			}
		}
		if rids := strArg(args, "related_ids"); rids != "" {
			for _, rid := range strings.Split(rids, ",") {
				if rid = strings.TrimSpace(rid); rid != "" {
					memory.RelatedIds = append(memory.RelatedIds, rid)
				}
			}
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.Store(callCtx, &ai_memorypb.StoreRqst{Memory: memory})
		if err != nil {
			return nil, fmt.Errorf("memory_store: %w", err)
		}

		return map[string]interface{}{
			"id":      rsp.GetId(),
			"status":  "stored",
			"project": memory.Project,
			"type":    memoryTypeString(memory.Type),
			"title":   memory.Title,
		}, nil
	})

	// ── memory_query ────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "memory_query",
		Description: "Search the AI knowledge base for relevant memories. Filter by project, " +
			"type, tags, or text search. Use this to recall past debugging sessions, " +
			"architecture decisions, user preferences, or any previously stored knowledge.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project": {Type: "string", Description: "Project identifier (required)"},
				"type": {Type: "string", Description: "Filter by memory type (optional)",
					Enum: []string{"feedback", "architecture", "decision", "debug", "session", "user", "project", "reference", "scratch"}},
				"tags":        {Type: "string", Description: "Comma-separated tags to filter by (AND logic)"},
				"text_search": {Type: "string", Description: "Substring search across title and content"},
				"limit":       {Type: "number", Description: "Max results (default 20)"},
			},
			Required: []string{"project"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		rqst := &ai_memorypb.QueryRqst{
			Project:    strArg(args, "project"),
			Type:       parseMemoryType(strArg(args, "type")),
			TextSearch: strArg(args, "text_search"),
			Limit:      20,
		}
		if tags := strArg(args, "tags"); tags != "" {
			for _, t := range strings.Split(tags, ",") {
				if t = strings.TrimSpace(t); t != "" {
					rqst.Tags = append(rqst.Tags, t)
				}
			}
		}
		if lim, ok := args["limit"].(float64); ok && lim > 0 {
			rqst.Limit = int32(lim)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.Query(callCtx, rqst)
		if err != nil {
			return nil, fmt.Errorf("memory_query: %w", err)
		}

		results := make([]map[string]interface{}, 0, len(rsp.GetMemories()))
		for _, m := range rsp.GetMemories() {
			entry := map[string]interface{}{
				"id":              m.GetId(),
				"type":            memoryTypeString(m.GetType()),
				"tags":            m.GetTags(),
				"title":           m.GetTitle(),
				"content":         m.GetContent(),
				"created_at":      fmtTimestamp(m.GetCreatedAt(), 0),
				"updated_at":      fmtTimestamp(m.GetUpdatedAt(), 0),
				"agent_id":        m.GetAgentId(),
				"reference_count": m.GetReferenceCount(),
			}
			if len(m.GetMetadata()) > 0 {
				entry["metadata"] = m.GetMetadata()
			}
			if len(m.GetRelatedIds()) > 0 {
				entry["related_ids"] = m.GetRelatedIds()
			}
			results = append(results, entry)
		}

		return map[string]interface{}{
			"total":    rsp.GetTotal(),
			"memories": results,
		}, nil
	})

	// ── memory_get ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "memory_get",
		Description: "Retrieve a single memory entry by its ID. Use when you have a specific memory ID from a previous query or session reference.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"id":      {Type: "string", Description: "Memory UUID"},
				"project": {Type: "string", Description: "Project identifier"},
			},
			Required: []string{"id", "project"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.Get(callCtx, &ai_memorypb.GetRqst{
			Id:      strArg(args, "id"),
			Project: strArg(args, "project"),
		})
		if err != nil {
			return nil, fmt.Errorf("memory_get: %w", err)
		}

		m := rsp.GetMemory()
		result := map[string]interface{}{
			"id":              m.GetId(),
			"type":            memoryTypeString(m.GetType()),
			"tags":            m.GetTags(),
			"title":           m.GetTitle(),
			"content":         m.GetContent(),
			"created_at":      fmtTimestamp(m.GetCreatedAt(), 0),
			"updated_at":      fmtTimestamp(m.GetUpdatedAt(), 0),
			"agent_id":        m.GetAgentId(),
			"conversation_id": m.GetConversationId(),
			"ttl_seconds":     m.GetTtlSeconds(),
			"reference_count": m.GetReferenceCount(),
		}
		if len(m.GetMetadata()) > 0 {
			result["metadata"] = m.GetMetadata()
		}
		if len(m.GetRelatedIds()) > 0 {
			result["related_ids"] = m.GetRelatedIds()
		}
		return result, nil
	})

	// ── memory_update ───────────────────────────────────────────────────
	s.register(toolDef{
		Name: "memory_update",
		Description: "Update an existing memory entry. Only non-empty fields are modified. " +
			"Use to correct, extend, or re-tag existing knowledge.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"id":          {Type: "string", Description: "Memory UUID to update"},
				"project":     {Type: "string", Description: "Project identifier"},
				"title":       {Type: "string", Description: "New title (optional)"},
				"content":     {Type: "string", Description: "New content (optional)"},
				"tags":        {Type: "string", Description: "New comma-separated tags (replaces existing)"},
				"ttl_seconds": {Type: "number", Description: "New TTL in seconds"},
				"metadata":    {Type: "string", Description: "JSON object of key-value pairs to merge into existing metadata"},
				"related_ids": {Type: "string", Description: "Comma-separated memory IDs to link (appended, deduplicated)"},
			},
			Required: []string{"id", "project"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		memory := &ai_memorypb.Memory{
			Id:        strArg(args, "id"),
			Project:   strArg(args, "project"),
			Title:     strArg(args, "title"),
			Content:   strArg(args, "content"),
			UpdatedAt: time.Now().Unix(),
		}
		if tags := strArg(args, "tags"); tags != "" {
			for _, t := range strings.Split(tags, ",") {
				if t = strings.TrimSpace(t); t != "" {
					memory.Tags = append(memory.Tags, t)
				}
			}
		}
		if ttl, ok := args["ttl_seconds"].(float64); ok {
			memory.TtlSeconds = int32(ttl)
		}
		if md := strArg(args, "metadata"); md != "" {
			parsed := make(map[string]string)
			if err := json.Unmarshal([]byte(md), &parsed); err == nil {
				memory.Metadata = parsed
			}
		}
		if rids := strArg(args, "related_ids"); rids != "" {
			for _, rid := range strings.Split(rids, ",") {
				if rid = strings.TrimSpace(rid); rid != "" {
					memory.RelatedIds = append(memory.RelatedIds, rid)
				}
			}
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.Update(callCtx, &ai_memorypb.UpdateRqst{Memory: memory})
		if err != nil {
			return nil, fmt.Errorf("memory_update: %w", err)
		}

		return map[string]interface{}{
			"success": rsp.GetSuccess(),
			"id":      memory.Id,
		}, nil
	})

	// ── memory_delete ───────────────────────────────────────────────────
	s.register(toolDef{
		Name: "memory_delete",
		Description: "Delete a memory entry by ID. Use to remove outdated, incorrect, " +
			"or superseded knowledge.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"id":      {Type: "string", Description: "Memory UUID to delete"},
				"project": {Type: "string", Description: "Project identifier"},
			},
			Required: []string{"id", "project"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.Delete(callCtx, &ai_memorypb.DeleteRqst{
			Id:      strArg(args, "id"),
			Project: strArg(args, "project"),
		})
		if err != nil {
			return nil, fmt.Errorf("memory_delete: %w", err)
		}

		return map[string]interface{}{
			"success": rsp.GetSuccess(),
			"id":      strArg(args, "id"),
		}, nil
	})

	// ── memory_list ─────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "memory_list",
		Description: "List memory summaries (without full content) for a project. " +
			"Lightweight view for browsing what knowledge is available before " +
			"fetching specific entries with memory_get.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project": {Type: "string", Description: "Project identifier"},
				"type": {Type: "string", Description: "Filter by memory type (optional)",
					Enum: []string{"feedback", "architecture", "decision", "debug", "session", "user", "project", "reference", "scratch"}},
				"tags":  {Type: "string", Description: "Comma-separated tags to filter by"},
				"limit": {Type: "number", Description: "Max results (default 20)"},
			},
			Required: []string{"project"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		rqst := &ai_memorypb.ListRqst{
			Project: strArg(args, "project"),
			Type:    parseMemoryType(strArg(args, "type")),
			Limit:   20,
		}
		if tags := strArg(args, "tags"); tags != "" {
			for _, t := range strings.Split(tags, ",") {
				if t = strings.TrimSpace(t); t != "" {
					rqst.Tags = append(rqst.Tags, t)
				}
			}
		}
		if lim, ok := args["limit"].(float64); ok && lim > 0 {
			rqst.Limit = int32(lim)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.List(callCtx, rqst)
		if err != nil {
			return nil, fmt.Errorf("memory_list: %w", err)
		}

		results := make([]map[string]interface{}, 0, len(rsp.GetMemories()))
		for _, m := range rsp.GetMemories() {
			results = append(results, map[string]interface{}{
				"id":         m.GetId(),
				"type":       memoryTypeString(m.GetType()),
				"tags":       m.GetTags(),
				"title":      m.GetTitle(),
				"created_at": fmtTimestamp(m.GetCreatedAt(), 0),
				"updated_at": fmtTimestamp(m.GetUpdatedAt(), 0),
			})
		}

		return map[string]interface{}{
			"total":    rsp.GetTotal(),
			"memories": results,
		}, nil
	})

	// ── session_save ────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "session_save",
		Description: "Save a conversation session summary for future continuity. " +
			"Call this at the end of a conversation to capture what was accomplished, " +
			"key decisions made, and open questions — so the next conversation can " +
			"pick up where this one left off.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project":          {Type: "string", Description: "Project identifier"},
				"topic":            {Type: "string", Description: "Main topic/area (e.g. 'dns-service-debugging', 'rbac-externalization')"},
				"summary":          {Type: "string", Description: "What was accomplished in this session"},
				"decisions":        {Type: "string", Description: "Comma-separated key decisions made"},
				"open_questions":   {Type: "string", Description: "Comma-separated unresolved items"},
				"related_memories": {Type: "string", Description: "Comma-separated memory IDs referenced or created"},
			},
			Required: []string{"project", "topic", "summary"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		session := &ai_memorypb.Session{
			Project:   strArg(args, "project"),
			Topic:     strArg(args, "topic"),
			Summary:   strArg(args, "summary"),
			CreatedAt: time.Now().Unix(),
			AgentId:   "claude-mcp",
		}
		if d := strArg(args, "decisions"); d != "" {
			for _, item := range strings.Split(d, ",") {
				if item = strings.TrimSpace(item); item != "" {
					session.Decisions = append(session.Decisions, item)
				}
			}
		}
		if q := strArg(args, "open_questions"); q != "" {
			for _, item := range strings.Split(q, ",") {
				if item = strings.TrimSpace(item); item != "" {
					session.OpenQuestions = append(session.OpenQuestions, item)
				}
			}
		}
		if r := strArg(args, "related_memories"); r != "" {
			for _, item := range strings.Split(r, ",") {
				if item = strings.TrimSpace(item); item != "" {
					session.RelatedMemories = append(session.RelatedMemories, item)
				}
			}
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.SaveSession(callCtx, &ai_memorypb.SaveSessionRqst{Session: session})
		if err != nil {
			return nil, fmt.Errorf("session_save: %w", err)
		}

		return map[string]interface{}{
			"id":      rsp.GetId(),
			"status":  "saved",
			"topic":   session.Topic,
			"project": session.Project,
		}, nil
	})

	// ── session_resume ──────────────────────────────────────────────────
	s.register(toolDef{
		Name: "session_resume",
		Description: "Retrieve previous session context for a topic. Use this at the " +
			"start of a conversation when the user references prior work, or when " +
			"you need to recall what was decided and what remains open.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"project": {Type: "string", Description: "Project identifier"},
				"topic":   {Type: "string", Description: "Topic to search for (fuzzy match on topic + summary)"},
				"limit":   {Type: "number", Description: "Number of recent sessions to return (default 1)"},
			},
			Required: []string{"project", "topic"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, memoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := ai_memorypb.NewAiMemoryServiceClient(conn)

		rqst := &ai_memorypb.ResumeSessionRqst{
			Project: strArg(args, "project"),
			Topic:   strArg(args, "topic"),
			Limit:   1,
		}
		if lim, ok := args["limit"].(float64); ok && lim > 0 {
			rqst.Limit = int32(lim)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.ResumeSession(callCtx, rqst)
		if err != nil {
			return nil, fmt.Errorf("session_resume: %w", err)
		}

		results := make([]map[string]interface{}, 0, len(rsp.GetSessions()))
		for _, s := range rsp.GetSessions() {
			results = append(results, map[string]interface{}{
				"id":               s.GetId(),
				"topic":            s.GetTopic(),
				"summary":          s.GetSummary(),
				"decisions":        s.GetDecisions(),
				"open_questions":   s.GetOpenQuestions(),
				"related_memories": s.GetRelatedMemories(),
				"created_at":       fmtTimestamp(s.GetCreatedAt(), 0),
				"agent_id":         s.GetAgentId(),
			})
		}

		return map[string]interface{}{
			"sessions": results,
			"count":    len(results),
		}, nil
	})
}

// strArg safely extracts a string argument from the tool args map.
func strArg(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

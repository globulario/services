package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// SendPrompt handles a conversational prompt — direct AI conversation with history.
func (srv *server) SendPrompt(req *ai_executorpb.SendPromptRequest, stream ai_executorpb.AiExecutorService_SendPromptServer) error {
	ctx := stream.Context()

	if req.Prompt == "" {
		return status.Error(codes.InvalidArgument, "prompt is required")
	}

	// Extract user ID from auth context if not provided.
	userID := req.UserId
	if userID == "" {
		userID = extractUserID(ctx)
	}
	if userID == "" {
		userID = "anonymous"
	}

	hostname, _ := os.Hostname()

	// Resolve the "__leader__" sentinel to the actual leader node hostname.
	if req.TargetNode == "__leader__" {
		resolved := srv.resolveLeaderHostname()
		if resolved != "" {
			req.TargetNode = resolved
		} else {
			// Could not determine leader — handle locally as best-effort.
			req.TargetNode = ""
		}
	}

	// Route to a specific peer if requested.
	// Ensure the user ID is on the request so the peer stores the
	// conversation under the correct user (not "anonymous").
	// Match both bare hostname and FQDN (hostname.domain) so that
	// the request is handled locally regardless of naming convention.
	if req.TargetNode != "" && !hostnameMatches(req.TargetNode, hostname, srv.Domain) {
		if req.UserId == "" {
			req.UserId = userID
		}
		return srv.proxyPromptToPeer(req, stream)
	}

	// Ensure conversation store is connected.
	if srv.convStore == nil || !srv.convStore.isConnected() {
		return status.Error(codes.Unavailable, "conversation store not available")
	}

	// Create or continue conversation.
	convID := req.ConversationId
	if convID == "" {
		// New conversation — generate title from first words of prompt.
		title := generateTitle(req.Prompt)
		var err error
		convID, err = srv.convStore.createConversation(userID, title, req.SystemPromptOverride)
		if err != nil {
			return status.Errorf(codes.Internal, "create conversation: %v", err)
		}
	}

	// Send thinking status.
	stream.Send(&ai_executorpb.SendPromptResponse{
		ConversationId: convID,
		Status:         ai_executorpb.ConversationStatus_CONV_STATUS_THINKING,
		RespondingNode: hostname,
	})

	// Save user message.
	userMsgID := uuid.New().String()
	if err := srv.convStore.saveMessage(convMessage{
		ID:             userMsgID,
		ConversationID: convID,
		Role:           "user",
		Content:        req.Prompt,
		CreatedAtMs:    time.Now().UnixMilli(),
	}); err != nil {
		logger.Warn("failed to save user message", "err", err)
	}

	// Load conversation history.
	history, err := srv.convStore.getMessages(convID, 50, 0)
	if err != nil {
		logger.Warn("failed to load history", "err", err)
		history = []convMessage{{Role: "user", Content: req.Prompt}}
	}

	// Build system prompt.
	systemPrompt := srv.convStore.getConversationSystemPrompt(convID)
	if req.SystemPromptOverride != "" {
		systemPrompt = req.SystemPromptOverride
	}
	if systemPrompt == "" {
		systemPrompt = defaultConversationPrompt(hostname)
	}

	// Convert history to API messages.
	apiMessages := historyToMessages(history)

	// Try to get a response from AI.
	var responseText string
	var inputTokens, outputTokens int

	if srv.diagnoser.anthropic != nil && srv.diagnoser.anthropic.isAvailable() {
		resp, apiErr := srv.diagnoser.anthropic.sendConversation(ctx, systemPrompt, apiMessages)
		if apiErr != nil {
			logger.Warn("anthropic API failed for conversation, trying CLI", "err", apiErr)
		} else {
			for _, block := range resp.Content {
				if block.Type == "text" && block.Text != "" {
					responseText += block.Text
				}
			}
			inputTokens = resp.Usage.InputTokens
			outputTokens = resp.Usage.OutputTokens
		}
	}

	// Fallback to CLI.
	if responseText == "" && srv.diagnoser.claude != nil && srv.diagnoser.claude.isAvailable() {
		// CLI is single-shot, so include recent history in prompt.
		cliPrompt := buildCLIConversationPrompt(systemPrompt, history)
		cliResp, cliErr := srv.diagnoser.claude.sendPrompt(ctx, cliPrompt)
		if cliErr != nil {
			logger.Warn("claude CLI failed for conversation", "err", cliErr)
			return status.Errorf(codes.Internal, "AI unavailable: %v", cliErr)
		}
		responseText = cliResp
	}

	if responseText == "" {
		return status.Error(codes.Unavailable, "no AI backend available")
	}

	// Save assistant message.
	assistantMsgID := uuid.New().String()
	if err := srv.convStore.saveMessage(convMessage{
		ID:             assistantMsgID,
		ConversationID: convID,
		Role:           "assistant",
		Content:        responseText,
		CreatedAtMs:    time.Now().UnixMilli(),
		NodeID:         srv.GetId(),
		NodeHostname:   hostname,
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
	}); err != nil {
		logger.Warn("failed to save assistant message", "err", err)
	}

	// Detect if AI is asking a question.
	needsReply := looksLikeQuestion(responseText)

	// Send the final response.
	return stream.Send(&ai_executorpb.SendPromptResponse{
		ConversationId: convID,
		MessageId:      assistantMsgID,
		FullText:       responseText,
		Status:         ai_executorpb.ConversationStatus_CONV_STATUS_COMPLETE,
		Done:           true,
		NeedsHumanReply:   needsReply,
		QuestionForHuman:  extractQuestion(responseText, needsReply),
		InputTokens:    int32(inputTokens),
		OutputTokens:   int32(outputTokens),
		RespondingNode: hostname,
	})
}

// GetConversation returns the message history for a conversation.
func (srv *server) GetConversation(ctx context.Context, req *ai_executorpb.GetConversationRequest) (*ai_executorpb.GetConversationResponse, error) {
	if srv.convStore == nil || !srv.convStore.isConnected() {
		return nil, status.Error(codes.Unavailable, "conversation store not available")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	messages, err := srv.convStore.getMessages(req.ConversationId, limit, req.BeforeMs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get messages: %v", err)
	}

	pbMessages := make([]*ai_executorpb.ConversationMessage, len(messages))
	for i, m := range messages {
		pbMessages[i] = &ai_executorpb.ConversationMessage{
			Id:             m.ID,
			ConversationId: m.ConversationID,
			Role:           m.Role,
			Content:        m.Content,
			CreatedAtMs:    m.CreatedAtMs,
			NodeId:         m.NodeID,
			NodeHostname:   m.NodeHostname,
			InputTokens:    int32(m.InputTokens),
			OutputTokens:   int32(m.OutputTokens),
			Metadata:       m.Metadata,
		}
	}

	title := srv.convStore.getConversationTitle(req.ConversationId)

	return &ai_executorpb.GetConversationResponse{
		ConversationId: req.ConversationId,
		Title:          title,
		Messages:       pbMessages,
	}, nil
}

// ListConversations returns a user's conversations.
func (srv *server) ListConversations(ctx context.Context, req *ai_executorpb.ListConversationsRequest) (*ai_executorpb.ListConversationsResponse, error) {
	if srv.convStore == nil || !srv.convStore.isConnected() {
		return nil, status.Error(codes.Unavailable, "conversation store not available")
	}

	userID := req.UserId
	if userID == "" {
		userID = extractUserID(ctx)
	}
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	summaries, err := srv.convStore.listConversations(userID, int(req.Limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list conversations: %v", err)
	}

	pbSummaries := make([]*ai_executorpb.ConversationSummary, len(summaries))
	for i, s := range summaries {
		pbSummaries[i] = &ai_executorpb.ConversationSummary{
			Id:                 s.ID,
			Title:              s.Title,
			UserId:             s.UserID,
			CreatedAtMs:        s.CreatedAtMs,
			UpdatedAtMs:        s.UpdatedAtMs,
			MessageCount:       int32(s.MessageCount),
			LastMessagePreview: s.LastMessagePreview,
		}
	}

	return &ai_executorpb.ListConversationsResponse{
		Conversations: pbSummaries,
	}, nil
}

// DeleteConversation removes a conversation and its messages.
func (srv *server) DeleteConversation(ctx context.Context, req *ai_executorpb.DeleteConversationRequest) (*ai_executorpb.DeleteConversationResponse, error) {
	if srv.convStore == nil || !srv.convStore.isConnected() {
		return nil, status.Error(codes.Unavailable, "conversation store not available")
	}

	if err := srv.convStore.deleteConversation(req.ConversationId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete conversation: %v", err)
	}

	return &ai_executorpb.DeleteConversationResponse{}, nil
}

// --- Peer proxying ---

// proxyPromptToPeer forwards a SendPrompt to a target peer node.
func (srv *server) proxyPromptToPeer(req *ai_executorpb.SendPromptRequest, stream ai_executorpb.AiExecutorService_SendPromptServer) error {
	peer := srv.findPeerByHostname(req.TargetNode)

	// If not found, trigger a re-discovery and try once more.
	// Peers may have come online after the last discovery cycle.
	if peer == nil {
		srv.peers.discoverPeers()
		peer = srv.findPeerByHostname(req.TargetNode)
	}

	if peer == nil {
		return status.Errorf(codes.NotFound, "peer node %q not found", req.TargetNode)
	}

	// Call the peer's SendPrompt and relay the stream.
	// Inject auth metadata so the peer's interceptor accepts the proxied call.
	peerCtx := peerAuthContext(stream.Context())
	peerStream, err := peer.Client.SendPrompt(peerCtx, req)
	if err != nil {
		return status.Errorf(codes.Internal, "proxy to peer %s: %v", req.TargetNode, err)
	}

	for {
		resp, err := peerStream.Recv()
		if err != nil {
			break
		}
		if sendErr := stream.Send(resp); sendErr != nil {
			return sendErr
		}
		if resp.Done {
			break
		}
	}
	return nil
}

// --- Helpers ---

// extractUserID gets the user identity from gRPC auth metadata.
func extractUserID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	// The interceptor sets "x-subject" from the JWT token.
	if vals := md.Get("x-subject"); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// generateTitle creates a conversation title from the first prompt.
func generateTitle(prompt string) string {
	// Take first ~50 chars, break at word boundary.
	title := prompt
	if len(title) > 50 {
		title = title[:50]
		if i := strings.LastIndex(title, " "); i > 20 {
			title = title[:i]
		}
		title += "..."
	}
	return title
}

// looksLikeQuestion detects if the AI response ends with a question.
func looksLikeQuestion(text string) bool {
	trimmed := strings.TrimSpace(text)
	if strings.HasSuffix(trimmed, "?") {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, marker := range []string{
		"would you like", "could you clarify", "can you provide",
		"shall i", "do you want", "please confirm", "let me know",
		"what would you prefer", "should i",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// extractQuestion pulls the last question from the response text.
func extractQuestion(text string, isQuestion bool) string {
	if !isQuestion {
		return ""
	}
	// Find the last sentence ending with '?'
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasSuffix(line, "?") {
			return line
		}
	}
	// Return last paragraph if no explicit question mark.
	if len(lines) > 0 {
		return strings.TrimSpace(lines[len(lines)-1])
	}
	return ""
}

// historyToMessages converts conversation history to Anthropic API format.
func historyToMessages(history []convMessage) []message {
	msgs := make([]message, 0, len(history))
	for _, h := range history {
		if h.Role != "user" && h.Role != "assistant" {
			continue
		}
		msgs = append(msgs, message{
			Role:    h.Role,
			Content: h.Content,
		})
	}
	return msgs
}

// buildCLIConversationPrompt builds a single prompt for the CLI that includes history.
func buildCLIConversationPrompt(systemPrompt string, history []convMessage) string {
	var b strings.Builder

	if systemPrompt != "" {
		b.WriteString(systemPrompt)
		b.WriteString("\n\n")
	}

	if len(history) > 1 {
		b.WriteString("## Conversation History\n\n")
		// Include all but the last message (which is the current prompt).
		for _, msg := range history[:len(history)-1] {
			if msg.Role == "user" {
				fmt.Fprintf(&b, "**User**: %s\n\n", msg.Content)
			} else {
				fmt.Fprintf(&b, "**Assistant**: %s\n\n", msg.Content)
			}
		}
		b.WriteString("## Current Message\n\n")
	}

	// Last message is the current user prompt.
	if len(history) > 0 {
		b.WriteString(history[len(history)-1].Content)
	}

	return b.String()
}

// defaultConversationPrompt returns the system prompt for conversations.
func defaultConversationPrompt(hostname string) string {
	return fmt.Sprintf(`You are the AI assistant for the Globular cluster node "%s".
You help operators manage, diagnose, and understand their distributed infrastructure.
You have access to MCP tools for cluster health, memory, RBAC, DNS, and service management.
Be helpful, concise, and specific. When you need more information, ask the user.
When suggesting actions that could affect the cluster, explain the impact first.`, hostname)
}

// findPeerByHostname searches the peer list for a node matching the given name.
func (srv *server) findPeerByHostname(target string) *peerConn {
	srv.peers.mu.RLock()
	defer srv.peers.mu.RUnlock()
	for _, p := range srv.peers.peers {
		if hostnameMatches(target, p.Hostname, srv.Domain) {
			return p
		}
	}
	return nil
}

// resolveLeaderHostname reads the cluster controller leader address from etcd
// and resolves it to a hostname. Returns empty string on failure.
func (srv *server) resolveLeaderHostname() string {
	addr, err := etcdGet("/globular/clustercontroller/leader/addr")
	if err != nil || addr == "" {
		logger.Warn("leader: could not read leader addr from etcd", "err", err)
		return ""
	}

	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		// addr might be a bare IP without port.
		ip = addr
	}

	// Check if the leader IP is local — if so, return empty so we handle locally.
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.String() == ip {
				return "" // local — handle here
			}
		}
	}

	// Try to resolve IP to hostname via /etc/hosts first.
	if name := resolveHostnameFromHosts(ip); name != "" {
		return name
	}

	// Try peer list — peers already have resolved hostnames.
	if srv.peers != nil {
		srv.peers.mu.RLock()
		for _, p := range srv.peers.peers {
			peerHost, _, _ := net.SplitHostPort(p.Endpoint)
			if peerHost == ip {
				srv.peers.mu.RUnlock()
				return p.Hostname
			}
		}
		srv.peers.mu.RUnlock()
	}

	// Last resort: return the IP itself — the routing logic will match it.
	return ip
}

// hostnameMatches returns true if target refers to the same host as hostname,
// considering bare hostnames and FQDNs (hostname.domain). For example,
// "globule-ryzen" matches "globule-ryzen.globular.internal" and vice versa.
func hostnameMatches(target, hostname, domain string) bool {
	if strings.EqualFold(target, hostname) {
		return true
	}
	fqdn := hostname
	if domain != "" && !strings.Contains(hostname, ".") {
		fqdn = hostname + "." + domain
	}
	if strings.EqualFold(target, fqdn) {
		return true
	}
	// Also match if target is an FQDN and hostname is bare.
	if domain != "" && strings.HasSuffix(strings.ToLower(target), "."+strings.ToLower(domain)) {
		bare := strings.TrimSuffix(strings.ToLower(target), "."+strings.ToLower(domain))
		if strings.EqualFold(bare, hostname) {
			return true
		}
	}
	return false
}

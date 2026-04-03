package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

const (
	conversationKeyspace = "ai_conversations"

	cqlCreateKeyspace = `CREATE KEYSPACE IF NOT EXISTS ai_conversations
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 3}`

	cqlCreateConversations = `CREATE TABLE IF NOT EXISTS ai_conversations.conversations (
		user_id         text,
		id              text,
		title           text,
		created_at_ms   bigint,
		updated_at_ms   bigint,
		message_count   int,
		last_message    text,
		system_prompt   text,
		metadata        map<text, text>,
		PRIMARY KEY ((user_id), updated_at_ms, id)
	) WITH CLUSTERING ORDER BY (updated_at_ms DESC, id ASC)`

	cqlCreateMessages = `CREATE TABLE IF NOT EXISTS ai_conversations.messages (
		conversation_id text,
		created_at_ms   bigint,
		id              text,
		role            text,
		content         text,
		node_id         text,
		node_hostname   text,
		input_tokens    int,
		output_tokens   int,
		metadata        map<text, text>,
		PRIMARY KEY ((conversation_id), created_at_ms, id)
	) WITH CLUSTERING ORDER BY (created_at_ms ASC, id ASC)`

	// Lookup table: conversation_id -> user_id (for deletion without knowing user_id)
	cqlCreateConvLookup = `CREATE TABLE IF NOT EXISTS ai_conversations.conv_lookup (
		conversation_id text PRIMARY KEY,
		user_id         text,
		title           text,
		system_prompt   text
	)`
)

type conversationStore struct {
	session *gocql.Session
}

func newConversationStore() *conversationStore {
	return &conversationStore{}
}

// connect establishes a ScyllaDB session and ensures schema exists.
func (cs *conversationStore) connect() error {
	hosts := scyllaHosts()
	if len(hosts) == 0 {
		return fmt.Errorf("no ScyllaDB hosts configured")
	}

	// Create keyspace first (connect without keyspace).
	cluster := gocql.NewCluster(hosts...)
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Consistency = gocql.Quorum

	initSession, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect: %w", err)
	}

	if err := initSession.Query(cqlCreateKeyspace).Exec(); err != nil {
		initSession.Close()
		return fmt.Errorf("create keyspace: %w", err)
	}
	initSession.Close()

	// Reconnect with keyspace.
	cluster.Keyspace = conversationKeyspace
	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect to keyspace: %w", err)
	}

	// Create tables.
	for _, ddl := range []string{cqlCreateConversations, cqlCreateMessages, cqlCreateConvLookup} {
		if err := session.Query(ddl).Exec(); err != nil {
			session.Close()
			return fmt.Errorf("create table: %w", err)
		}
	}

	cs.session = session
	logger.Info("conversation_store: connected to ScyllaDB", "hosts", hosts)
	return nil
}

func (cs *conversationStore) close() {
	if cs.session != nil {
		cs.session.Close()
	}
}

func (cs *conversationStore) isConnected() bool {
	return cs.session != nil && !cs.session.Closed()
}

// scyllaHosts returns ScyllaDB contact points.
// Checks env, then ScyllaDB config, then local network interfaces.
func scyllaHosts() []string {
	if h := os.Getenv("SCYLLA_HOSTS"); h != "" {
		return strings.Split(h, ",")
	}

	// Read ScyllaDB listen address from its config file.
	if data, err := os.ReadFile("/etc/scylla/scylla.yaml"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "listen_address:") {
				addr := strings.TrimSpace(strings.TrimPrefix(line, "listen_address:"))
				addr = strings.Trim(addr, "'\"")
				if addr != "" && addr != "localhost" {
					return []string{addr}
				}
			}
		}
	}

	// Fall back to the local IP from config.
	if localIP := config.GetLocalIP(); localIP != "" {
		return []string{localIP}
	}

	return []string{"127.0.0.1"}
}

// --- Conversation CRUD ---

type convMessage struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	CreatedAtMs    int64
	NodeID         string
	NodeHostname   string
	InputTokens    int
	OutputTokens   int
	Metadata       map[string]string
}

type convSummary struct {
	ID                 string
	Title              string
	UserID             string
	CreatedAtMs        int64
	UpdatedAtMs        int64
	MessageCount       int
	LastMessagePreview string
}

// createConversation creates a new conversation and returns its ID.
func (cs *conversationStore) createConversation(userID, title, systemPrompt string) (string, error) {
	id := uuid.New().String()
	now := time.Now().UnixMilli()

	batch := cs.session.NewBatch(gocql.LoggedBatch)
	batch.Query(`INSERT INTO conversations (user_id, id, title, created_at_ms, updated_at_ms, message_count, last_message, system_prompt)
		VALUES (?, ?, ?, ?, ?, 0, '', ?)`, userID, id, title, now, now, systemPrompt)
	batch.Query(`INSERT INTO conv_lookup (conversation_id, user_id, title, system_prompt)
		VALUES (?, ?, ?, ?)`, id, userID, title, systemPrompt)

	if err := cs.session.ExecuteBatch(batch); err != nil {
		return "", fmt.Errorf("create conversation: %w", err)
	}
	return id, nil
}

// saveMessage persists a message and updates the conversation metadata.
func (cs *conversationStore) saveMessage(msg convMessage) error {
	// Get user_id from lookup.
	var userID string
	if err := cs.session.Query(`SELECT user_id FROM conv_lookup WHERE conversation_id = ?`,
		msg.ConversationID).Scan(&userID); err != nil {
		return fmt.Errorf("lookup conversation: %w", err)
	}

	now := time.Now().UnixMilli()
	if msg.CreatedAtMs == 0 {
		msg.CreatedAtMs = now
	}
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	preview := msg.Content
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}

	batch := cs.session.NewBatch(gocql.LoggedBatch)
	batch.Query(`INSERT INTO messages (conversation_id, created_at_ms, id, role, content, node_id, node_hostname, input_tokens, output_tokens, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ConversationID, msg.CreatedAtMs, msg.ID, msg.Role, msg.Content,
		msg.NodeID, msg.NodeHostname, msg.InputTokens, msg.OutputTokens, msg.Metadata)

	// Update conversation metadata. We need to delete old row and insert new
	// since updated_at_ms is part of the clustering key.
	// Use a simple UPDATE on the counter and last_message instead — but CQL
	// doesn't allow updating clustering keys. So we just insert a new row;
	// the listing query returns the latest by clustering order.
	batch.Query(`INSERT INTO conversations (user_id, id, title, created_at_ms, updated_at_ms, message_count, last_message)
		VALUES (?, ?, '', 0, ?, 0, ?)`, userID, msg.ConversationID, now, preview)

	return cs.session.ExecuteBatch(batch)
}

// getMessages returns messages for a conversation, ordered by time.
func (cs *conversationStore) getMessages(conversationID string, limit int, beforeMs int64) ([]convMessage, error) {
	var query string
	var args []interface{}

	if beforeMs > 0 {
		query = `SELECT id, conversation_id, role, content, created_at_ms, node_id, node_hostname, input_tokens, output_tokens, metadata
			FROM messages WHERE conversation_id = ? AND created_at_ms < ? ORDER BY created_at_ms ASC`
		args = []interface{}{conversationID, beforeMs}
	} else {
		query = `SELECT id, conversation_id, role, content, created_at_ms, node_id, node_hostname, input_tokens, output_tokens, metadata
			FROM messages WHERE conversation_id = ? ORDER BY created_at_ms ASC`
		args = []interface{}{conversationID}
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	iter := cs.session.Query(query, args...).Iter()
	var messages []convMessage
	var msg convMessage
	for iter.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.CreatedAtMs,
		&msg.NodeID, &msg.NodeHostname, &msg.InputTokens, &msg.OutputTokens, &msg.Metadata) {
		messages = append(messages, msg)
		msg = convMessage{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	return messages, nil
}

// listConversations returns conversation summaries for a user.
func (cs *conversationStore) listConversations(userID string, limit int) ([]convSummary, error) {
	if limit <= 0 {
		limit = 20
	}

	// Get unique conversation IDs (latest entry per conversation).
	iter := cs.session.Query(`SELECT id, title, updated_at_ms, message_count, last_message
		FROM conversations WHERE user_id = ? LIMIT ?`, userID, limit*2).Iter()

	seen := make(map[string]bool)
	var summaries []convSummary
	var id, title, lastMsg string
	var updatedAt int64
	var msgCount int

	for iter.Scan(&id, &title, &updatedAt, &msgCount, &lastMsg) {
		if seen[id] {
			continue
		}
		seen[id] = true

		// Get the real title from lookup if empty.
		if title == "" {
			cs.session.Query(`SELECT title FROM conv_lookup WHERE conversation_id = ?`, id).Scan(&title)
		}

		summaries = append(summaries, convSummary{
			ID:                 id,
			Title:              title,
			UserID:             userID,
			UpdatedAtMs:        updatedAt,
			MessageCount:       msgCount,
			LastMessagePreview: lastMsg,
		})

		if len(summaries) >= limit {
			break
		}
	}
	iter.Close()
	return summaries, nil
}

// getConversationTitle returns the title for a conversation.
func (cs *conversationStore) getConversationTitle(conversationID string) string {
	var title string
	cs.session.Query(`SELECT title FROM conv_lookup WHERE conversation_id = ?`, conversationID).Scan(&title)
	return title
}

// getConversationSystemPrompt returns the custom system prompt if set.
func (cs *conversationStore) getConversationSystemPrompt(conversationID string) string {
	var sp string
	cs.session.Query(`SELECT system_prompt FROM conv_lookup WHERE conversation_id = ?`, conversationID).Scan(&sp)
	return sp
}

// deleteConversation removes a conversation and all its messages.
func (cs *conversationStore) deleteConversation(conversationID string) error {
	// Get user_id from lookup.
	var userID string
	if err := cs.session.Query(`SELECT user_id FROM conv_lookup WHERE conversation_id = ?`,
		conversationID).Scan(&userID); err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}

	batch := cs.session.NewBatch(gocql.LoggedBatch)
	batch.Query(`DELETE FROM messages WHERE conversation_id = ?`, conversationID)
	batch.Query(`DELETE FROM conv_lookup WHERE conversation_id = ?`, conversationID)

	if err := cs.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}

	// Delete conversation entries from the user's list.
	// We need to find all rows for this conversation_id under this user.
	iter := cs.session.Query(`SELECT updated_at_ms FROM conversations WHERE user_id = ?`, userID).Iter()
	var updatedAt int64
	for iter.Scan(&updatedAt) {
		cs.session.Query(`DELETE FROM conversations WHERE user_id = ? AND updated_at_ms = ? AND id = ?`,
			userID, updatedAt, conversationID).Exec()
	}
	iter.Close()

	return nil
}

package incidentpattern

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/google/uuid"
)

// ackFile is the on-disk format for an incident acknowledgement.
type ackFile struct {
	ID                 string `json:"id"`
	SessionID          string `json:"session_id"`
	IncidentID         string `json:"incident_id"`
	AcknowledgedReason string `json:"acknowledged_reason"`
	CreatedAt          int64  `json:"created_at"`
}

// sharedAckStore is the in-memory backing shared across all AcknowledgementStore
// instances for the same Graph when dataDir == "". Stored in Graph.MemRegistry().
type sharedAckStore struct {
	mu   sync.Mutex
	acks map[string]bool // key: sessionID + "|" + incidentID
}

func sharedAcks(g *graph.Graph) *sharedAckStore {
	v, _ := g.MemRegistry().LoadOrStore("incident_acks", &sharedAckStore{
		acks: make(map[string]bool),
	})
	return v.(*sharedAckStore)
}

// AcknowledgementStore persists per-session incident acknowledgements.
type AcknowledgementStore struct {
	dataDir string // <graph.DataDir()>/incident_acks; "" = in-memory

	// shared is used when dataDir == "" — points to graph-level shared storage.
	shared *sharedAckStore
}

// NewAcknowledgementStore returns an AcknowledgementStore backed by the awareness graph.
func NewAcknowledgementStore(g *graph.Graph) *AcknowledgementStore {
	dir := ""
	if d := g.DataDir(); d != "" {
		dir = filepath.Join(d, "incident_acks")
	}
	s := &AcknowledgementStore{dataDir: dir}
	if dir == "" {
		s.shared = sharedAcks(g)
	}
	return s
}

func (a *AcknowledgementStore) acksDir() string {
	if a.dataDir == "" {
		return ""
	}
	_ = os.MkdirAll(a.dataDir, 0o755)
	return a.dataDir
}

// AcknowledgeIncident records that the agent has read the incident and is proceeding
// with an adjusted plan.
func (a *AcknowledgementStore) AcknowledgeIncident(ctx context.Context, sessionID, incidentID, reason string) error {
	if a.dataDir == "" {
		// In-memory mode — use shared graph-level storage.
		a.shared.mu.Lock()
		a.shared.acks[sessionID+"|"+incidentID] = true
		a.shared.mu.Unlock()
		return nil
	}

	id := uuid.New().String()
	rec := &ackFile{
		ID:                 id,
		SessionID:          sessionID,
		IncidentID:         incidentID,
		AcknowledgedReason: reason,
		CreatedAt:          time.Now().Unix(),
	}

	dir := a.acksDir()
	if dir == "" {
		return nil
	}

	// File name: <session>_<incident>_<uuid8>.json
	filename := sanitizeAckID(sessionID) + "_" + sanitizeAckID(incidentID) + "_" + id[:8] + ".json"
	path := filepath.Join(dir, filename)

	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("incidentpattern: acknowledge %s: %w", incidentID, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("incidentpattern: acknowledge %s: %w", incidentID, err)
	}
	return nil
}

// IsAcknowledgedInSession returns true when the agent has already acknowledged
// this incident in the current session.
func (a *AcknowledgementStore) IsAcknowledgedInSession(ctx context.Context, sessionID, incidentID string) bool {
	if a.dataDir == "" {
		// In-memory mode — use shared graph-level storage.
		a.shared.mu.Lock()
		v := a.shared.acks[sessionID+"|"+incidentID]
		a.shared.mu.Unlock()
		return v
	}

	dir := a.acksDir()
	if dir == "" {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	// Files are named <sessionID>_<incidentID>_<uuid8>.json.
	prefix := sanitizeAckID(sessionID) + "_" + sanitizeAckID(incidentID) + "_"
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".json") {
			return true
		}
	}
	return false
}

func sanitizeAckID(id string) string {
	r := strings.NewReplacer("/", "_", ":", "_", " ", "_", ".", "_", "-", "_")
	return r.Replace(id)
}

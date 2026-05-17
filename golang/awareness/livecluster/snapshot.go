package livecluster

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

// snapshotFile is the on-disk format for a ClusterSignalSnapshot.
// Sub-records (services, errors, convergence, incidents) are embedded.
type snapshotFile struct {
	ID               string                    `json:"id"`
	ClusterID        string                    `json:"cluster_id"`
	NodeID           string                    `json:"node_id"`
	CollectedAt      int64                     `json:"collected_at"`
	CollectorVersion string                    `json:"collector_version"`
	Status           string                    `json:"status"`
	Summary          string                    `json:"summary"`
	Sources          []SignalSourceStatus      `json:"sources,omitempty"`
	Services         []ServiceLiveState        `json:"services,omitempty"`
	Errors           []RecentErrorSignature    `json:"errors,omitempty"`
	Convergence      []RuntimeConvergenceState `json:"convergence,omitempty"`
	Incidents        []ActiveClusterIncident   `json:"incidents,omitempty"`
}

// preflightFile is the on-disk format for a LivePreflightResult.
type preflightFile struct {
	ID               string                 `json:"id"`
	SessionID        string                 `json:"session_id"`
	Task             string                 `json:"task"`
	Files            []string               `json:"files,omitempty"`
	Components       []string               `json:"components,omitempty"`
	StaticResultID   string                 `json:"static_result_id"`
	SignalSnapshotID string                 `json:"signal_snapshot_id"`
	Verdict          string                 `json:"verdict"`
	Severity         string                 `json:"severity"`
	Summary          string                 `json:"summary"`
	Blockers         []LivePreflightFinding `json:"blockers,omitempty"`
	Warnings         []LivePreflightFinding `json:"warnings,omitempty"`
	Confirmations    []LivePreflightFinding `json:"confirmations,omitempty"`
	CreatedAt        int64                  `json:"created_at"`
}

// Store persists and retrieves live cluster signal data.
type Store struct {
	mu      sync.Mutex
	dataDir string // base data directory from graph; "" = in-memory

	// In-memory maps used when dataDir == "".
	memSnapshots  map[string]*snapshotFile   // id → snapshot
	memPreflights map[string]*preflightFile  // id → preflight
}

// NewStore returns a Store backed by the awareness graph.
func NewStore(g *graph.Graph) *Store {
	return &Store{
		dataDir:       g.DataDir(),
		memSnapshots:  make(map[string]*snapshotFile),
		memPreflights: make(map[string]*preflightFile),
	}
}

func (s *Store) snapshotsDir(clusterID string) string {
	if s.dataDir == "" {
		return ""
	}
	d := filepath.Join(s.dataDir, "cluster_snapshots", sanitizeClusterID(clusterID))
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (s *Store) preflightDir() string {
	if s.dataDir == "" {
		return ""
	}
	d := filepath.Join(s.dataDir, "live_preflight")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func sanitizeClusterID(id string) string {
	r := strings.NewReplacer("/", "_", ":", "_", " ", "_", ".", "_")
	return r.Replace(id)
}

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// StoreClusterSignalSnapshot persists a full snapshot and all its sub-records.
func (s *Store) StoreClusterSignalSnapshot(ctx context.Context, snap *ClusterSignalSnapshot) error {
	if snap.ID == "" {
		snap.ID = uuid.New().String()
	}

	sf := &snapshotFile{
		ID:               snap.ID,
		ClusterID:        snap.ClusterID,
		NodeID:           snap.NodeID,
		CollectedAt:      snap.CollectedAt,
		CollectorVersion: snap.CollectorVersion,
		Status:           snap.Status,
		Summary:          snap.Summary,
		Sources:          snap.Sources,
		Services:         snap.Services,
		Errors:           snap.Errors,
		Convergence:      snap.Convergence,
		Incidents:        snap.Incidents,
	}

	if s.dataDir == "" {
		s.mu.Lock()
		cp := *sf
		s.memSnapshots[sf.ID] = &cp
		s.mu.Unlock()
		return nil
	}

	dir := s.snapshotsDir(snap.ClusterID)
	if dir == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONAtomic(filepath.Join(dir, sanitizeClusterID(snap.ID)+".json"), sf)
}

// GetLatestClusterSignalSnapshot loads the most recent snapshot for a cluster.
func (s *Store) GetLatestClusterSignalSnapshot(ctx context.Context, clusterID string) (*ClusterSignalSnapshot, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		var latest *snapshotFile
		for _, sf := range s.memSnapshots {
			if sf.ClusterID != clusterID {
				continue
			}
			if latest == nil || sf.CollectedAt > latest.CollectedAt {
				r := *sf
				latest = &r
			}
		}
		if latest == nil {
			return nil, fmt.Errorf("no snapshot for cluster %s", clusterID)
		}
		return snapshotFileToSnapshot(latest), nil
	}

	dir := s.snapshotsDir(clusterID)
	if dir == "" {
		return nil, fmt.Errorf("no snapshot for cluster %s", clusterID)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no snapshot for cluster %s", clusterID)
		}
		return nil, err
	}

	var latest *snapshotFile
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var sf snapshotFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}
		if latest == nil || sf.CollectedAt > latest.CollectedAt {
			r := sf
			latest = &r
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no snapshot for cluster %s", clusterID)
	}

	return snapshotFileToSnapshot(latest), nil
}

func snapshotFileToSnapshot(latest *snapshotFile) *ClusterSignalSnapshot {
	return &ClusterSignalSnapshot{
		ID:               latest.ID,
		ClusterID:        latest.ClusterID,
		NodeID:           latest.NodeID,
		CollectedAt:      latest.CollectedAt,
		CollectorVersion: latest.CollectorVersion,
		Status:           latest.Status,
		Summary:          latest.Summary,
		Sources:          latest.Sources,
		Services:         latest.Services,
		Errors:           latest.Errors,
		Convergence:      latest.Convergence,
		Incidents:        latest.Incidents,
	}
}

// StoreLivePreflightResult persists a live preflight verdict.
func (s *Store) StoreLivePreflightResult(ctx context.Context, r *LivePreflightResult) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}

	pf := &preflightFile{
		ID:               r.ID,
		SessionID:        r.SessionID,
		Task:             r.Task,
		Files:            r.Files,
		Components:       r.Components,
		StaticResultID:   r.StaticResultID,
		SignalSnapshotID: r.SignalSnapshotID,
		Verdict:          r.Verdict,
		Severity:         r.Severity,
		Summary:          r.Summary,
		Blockers:         r.Blockers,
		Warnings:         r.Warnings,
		Confirmations:    r.Confirmations,
		CreatedAt:        time.Now().Unix(),
	}

	if s.dataDir == "" {
		s.mu.Lock()
		cp := *pf
		s.memPreflights[pf.ID] = &cp
		s.mu.Unlock()
		return nil
	}

	dir := s.preflightDir()
	if dir == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONAtomic(filepath.Join(dir, sanitizeClusterID(r.ID)+".json"), pf)
}

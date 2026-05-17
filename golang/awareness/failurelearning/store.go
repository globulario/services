package failurelearning

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
)

// Store wraps JSON file persistence for the three failure_learning_* tables.
type Store struct {
	mu      sync.Mutex
	dataDir string // base data directory from graph; "" = in-memory

	// In-memory maps used when dataDir == "".
	memProposals map[string]*FailureLearningProposal
	memReviews   map[string]*FailureLearningReview
	memSeedSyncs map[string]*SeedSyncStatus
}

// New creates a Store from a *graph.Graph.
func New(g *graph.Graph) *Store {
	return &Store{
		dataDir:      g.DataDir(),
		memProposals: make(map[string]*FailureLearningProposal),
		memReviews:   make(map[string]*FailureLearningReview),
		memSeedSyncs: make(map[string]*SeedSyncStatus),
	}
}

// randomHex returns n random hex bytes as a lowercase string.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newProposalID returns a unique proposal ID.
func newProposalID() string {
	return fmt.Sprintf("FLP-%d-%s", time.Now().UnixMilli(), randomHex(4))
}

// newReviewID returns a unique review ID.
func newReviewID() string {
	return fmt.Sprintf("FLREV-%d-%s", time.Now().UnixMilli(), randomHex(4))
}

// newSeedSyncID returns a unique seed sync ID.
func newSeedSyncID() string {
	return fmt.Sprintf("FLSEED-%d-%s", time.Now().UnixMilli(), randomHex(4))
}

func sanitizeID(id string) string {
	r := strings.NewReplacer("/", "_", ":", "_", " ", "_", ".", "_")
	return r.Replace(id)
}

func (s *Store) subdirFor(kind string) string {
	if s.dataDir == "" {
		return ""
	}
	d := filepath.Join(s.dataDir, "failure_learning", kind)
	_ = os.MkdirAll(d, 0o755)
	return d
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

func (s *Store) writeTo(kind, id string, v any) error {
	dir := s.subdirFor(kind)
	if dir == "" {
		return nil // in-memory only
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONAtomic(filepath.Join(dir, sanitizeID(id)+".json"), v)
}

func (s *Store) readFrom(kind, id string, v any) error {
	dir := s.subdirFor(kind)
	if dir == "" {
		return fmt.Errorf("failurelearning: in-memory graph, no record %s/%s", kind, id)
	}
	data, err := os.ReadFile(filepath.Join(dir, sanitizeID(id)+".json"))
	if err != nil {
		return fmt.Errorf("failurelearning: %s %s not found: %w", kind, id, err)
	}
	return json.Unmarshal(data, v)
}

func (s *Store) listFrom(kind string) ([][]byte, error) {
	dir := s.subdirFor(kind)
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out [][]byte
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, data)
	}
	return out, nil
}

func (s *Store) updateProposal(id string, fn func(*FailureLearningProposal)) error {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		p, ok := s.memProposals[id]
		if !ok {
			return fmt.Errorf("failurelearning: proposal %s not found", id)
		}
		fn(p)
		return nil
	}
	var p FailureLearningProposal
	if err := s.readFrom("proposals", id, &p); err != nil {
		return err
	}
	fn(&p)
	return s.writeTo("proposals", id, &p)
}

// SaveProposal upserts a FailureLearningProposal. Assigns a new ID if empty.
func (s *Store) SaveProposal(ctx context.Context, p FailureLearningProposal) (*FailureLearningProposal, error) {
	if p.ID == "" {
		p.ID = newProposalID()
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().UnixMilli()
	}
	if s.dataDir == "" {
		s.mu.Lock()
		cp := p
		s.memProposals[p.ID] = &cp
		s.mu.Unlock()
		return &p, nil
	}
	if err := s.writeTo("proposals", p.ID, &p); err != nil {
		return nil, fmt.Errorf("failurelearning: save proposal: %w", err)
	}
	return &p, nil
}

// GetProposal retrieves a proposal by ID.
func (s *Store) GetProposal(ctx context.Context, id string) (*FailureLearningProposal, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		p, ok := s.memProposals[id]
		s.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("failurelearning: proposal %s not found", id)
		}
		cp := *p
		return &cp, nil
	}
	var p FailureLearningProposal
	if err := s.readFrom("proposals", id, &p); err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("failurelearning: proposal %s not found", id)
		}
		return nil, fmt.Errorf("failurelearning: get proposal %s: %w", id, err)
	}
	return &p, nil
}

// UpdateProposalStatus updates the status, reviewedBy, and reviewedAt fields.
func (s *Store) UpdateProposalStatus(ctx context.Context, id, status, reviewedBy string, reviewedAt int64) error {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		p, ok := s.memProposals[id]
		if !ok {
			return fmt.Errorf("failurelearning: proposal %s not found", id)
		}
		p.Status = status
		p.ReviewedBy = reviewedBy
		p.ReviewedAt = reviewedAt
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var p FailureLearningProposal
	dir := s.subdirFor("proposals")
	if dir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, sanitizeID(id)+".json"))
	if err != nil {
		return fmt.Errorf("failurelearning: proposal %s not found", id)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.Status = status
	p.ReviewedBy = reviewedBy
	p.ReviewedAt = reviewedAt
	return writeJSONAtomic(filepath.Join(dir, sanitizeID(id)+".json"), &p)
}

// MarkApplied stamps applied_at on the proposal and sets status=applied.
func (s *Store) MarkApplied(ctx context.Context, id string, appliedAt int64) error {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		p, ok := s.memProposals[id]
		if !ok {
			return fmt.Errorf("failurelearning: proposal %s not found", id)
		}
		p.Status = "applied"
		p.AppliedAt = appliedAt
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.subdirFor("proposals")
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, sanitizeID(id)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failurelearning: proposal %s not found", id)
	}
	var p FailureLearningProposal
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	p.Status = "applied"
	p.AppliedAt = appliedAt
	return writeJSONAtomic(path, &p)
}

// ListPending returns all proposals with status=proposed.
func (s *Store) ListPending(ctx context.Context) ([]FailureLearningProposal, error) {
	all, err := s.listAllProposals()
	if err != nil {
		return nil, err
	}
	var out []FailureLearningProposal
	for _, p := range all {
		if p.Status == "proposed" {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// ListBySource returns all proposals for the given source_type and source_id.
func (s *Store) ListBySource(ctx context.Context, sourceType, sourceID string) ([]FailureLearningProposal, error) {
	all, err := s.listAllProposals()
	if err != nil {
		return nil, err
	}
	var out []FailureLearningProposal
	for _, p := range all {
		if p.SourceType == sourceType && p.SourceID == sourceID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func (s *Store) listAllProposals() ([]FailureLearningProposal, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		var out []FailureLearningProposal
		for _, p := range s.memProposals {
			out = append(out, *p)
		}
		return out, nil
	}
	blobs, err := s.listFrom("proposals")
	if err != nil {
		return nil, fmt.Errorf("failurelearning: query proposals: %w", err)
	}
	var out []FailureLearningProposal
	for _, data := range blobs {
		var p FailureLearningProposal
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// SaveReview upserts a FailureLearningReview, assigning an ID if empty.
func (s *Store) SaveReview(ctx context.Context, r FailureLearningReview) (*FailureLearningReview, error) {
	if r.ID == "" {
		r.ID = newReviewID()
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = time.Now().UnixMilli()
	}
	if s.dataDir == "" {
		s.mu.Lock()
		cp := r
		s.memReviews[r.ID] = &cp
		s.mu.Unlock()
		return &r, nil
	}
	if err := s.writeTo("reviews", r.ID, &r); err != nil {
		return nil, fmt.Errorf("failurelearning: save review: %w", err)
	}
	return &r, nil
}

// SaveSeedSync upserts a SeedSyncStatus record, assigning an ID if empty.
func (s *Store) SaveSeedSync(ctx context.Context, ss SeedSyncStatus) error {
	if ss.ID == "" {
		ss.ID = newSeedSyncID()
	}
	now := time.Now().UnixMilli()
	if ss.CreatedAt == 0 {
		ss.CreatedAt = now
	}
	ss.UpdatedAt = now
	if s.dataDir == "" {
		s.mu.Lock()
		cp := ss
		s.memSeedSyncs[ss.ID] = &cp
		s.mu.Unlock()
		return nil
	}
	if err := s.writeTo("seed_sync", ss.ID, &ss); err != nil {
		return fmt.Errorf("failurelearning: save seed sync: %w", err)
	}
	return nil
}

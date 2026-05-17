package incidentpattern

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/google/uuid"
)

// patternFile is the on-disk format for an IncidentPattern with all relations embedded.
type patternFile struct {
	ID          string              `json:"id"`
	IncidentID  string              `json:"incident_id"`
	Title       string              `json:"title"`
	Summary     string              `json:"summary"`
	Severity    string              `json:"severity"`
	Status      string              `json:"status"`
	FailureMode string              `json:"failure_mode"`
	RootCause   string              `json:"root_cause"`
	Lesson      string              `json:"lesson"`
	CreatedAt   int64               `json:"created_at"`
	UpdatedAt   int64               `json:"updated_at"`
	Files       []PatternFile       `json:"files,omitempty"`
	Symbols     []PatternSymbol     `json:"symbols,omitempty"`
	Invariants  []PatternInvariant  `json:"invariants,omitempty"`
	FailedFixes []FailedFix         `json:"failed_fixes,omitempty"`
	EditShapes  []EditShape         `json:"edit_shapes,omitempty"`
	Proposals   []PatternProposal   `json:"proposals,omitempty"`
}

// sharedPatternStore is the in-memory backing shared across all Store instances
// for the same Graph when dataDir == "". Stored in Graph.MemRegistry().
type sharedPatternStore struct {
	mu       sync.Mutex
	patterns map[string]*patternFile
}

func sharedPatterns(g *graph.Graph) *sharedPatternStore {
	v, _ := g.MemRegistry().LoadOrStore("incidentpattern", &sharedPatternStore{
		patterns: make(map[string]*patternFile),
	})
	return v.(*sharedPatternStore)
}

// Store provides persistence for incident patterns backed by JSON files
// or an in-memory map when the graph has no data directory.
type Store struct {
	dataDir string // <graph.DataDir()>/incident_patterns; "" = in-memory

	// shared is used when dataDir == "" — points to graph-level shared storage
	// so multiple Store instances on the same Graph share the same patterns.
	shared *sharedPatternStore
}

// NewStore returns a Store backed by the given awareness graph.
func NewStore(g *graph.Graph) *Store {
	dir := ""
	if d := g.DataDir(); d != "" {
		dir = filepath.Join(d, "incident_patterns")
	}
	s := &Store{dataDir: dir}
	if dir == "" {
		s.shared = sharedPatterns(g)
	}
	return s
}

func (s *Store) patternsDir() string {
	if s.dataDir == "" {
		return ""
	}
	_ = os.MkdirAll(s.dataDir, 0o755)
	return s.dataDir
}

func (s *Store) patternPath(id string) string {
	return filepath.Join(s.patternsDir(), sanitizeID(id)+".json")
}

func sanitizeID(id string) string {
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

func (s *Store) writePattern(pf *patternFile) error {
	if s.dataDir == "" {
		// In-memory mode — use shared graph-level storage.
		s.shared.mu.Lock()
		cp := *pf
		s.shared.patterns[pf.ID] = &cp
		s.shared.mu.Unlock()
		return nil
	}

	dir := s.patternsDir()
	if dir == "" {
		return nil
	}
	return writeJSONAtomic(s.patternPath(pf.ID), pf)
}

func (s *Store) readPattern(id string) (*patternFile, error) {
	if s.dataDir == "" {
		// In-memory mode — use shared graph-level storage.
		s.shared.mu.Lock()
		pf, ok := s.shared.patterns[id]
		s.shared.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("incidentpattern: in-memory graph, no pattern %s", id)
		}
		cp := *pf
		return &cp, nil
	}

	dir := s.patternsDir()
	if dir == "" {
		return nil, fmt.Errorf("incidentpattern: in-memory graph, no pattern %s", id)
	}
	data, err := os.ReadFile(s.patternPath(id))
	if err != nil {
		return nil, fmt.Errorf("incidentpattern: load %s: %w", id, err)
	}
	var pf patternFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("incidentpattern: decode %s: %w", id, err)
	}
	return &pf, nil
}

func patternFileToIncidentPattern(pf *patternFile) *IncidentPattern {
	p := &IncidentPattern{
		ID:          pf.ID,
		IncidentID:  pf.IncidentID,
		Title:       pf.Title,
		Summary:     pf.Summary,
		Severity:    pf.Severity,
		Status:      pf.Status,
		FailureMode: pf.FailureMode,
		RootCause:   pf.RootCause,
		Lesson:      pf.Lesson,
		CreatedAt:   pf.CreatedAt,
		UpdatedAt:   pf.UpdatedAt,
		Files:       pf.Files,
		Symbols:     pf.Symbols,
		Invariants:  pf.Invariants,
		FailedFixes: pf.FailedFixes,
		EditShapes:  pf.EditShapes,
		Proposals:   pf.Proposals,
	}
	// Ensure PatternID is set on sub-records.
	for i := range p.Files {
		p.Files[i].PatternID = pf.ID
	}
	for i := range p.Symbols {
		p.Symbols[i].PatternID = pf.ID
	}
	for i := range p.Invariants {
		p.Invariants[i].PatternID = pf.ID
	}
	for i := range p.FailedFixes {
		p.FailedFixes[i].PatternID = pf.ID
	}
	for i := range p.EditShapes {
		p.EditShapes[i].PatternID = pf.ID
	}
	for i := range p.Proposals {
		p.Proposals[i].PatternID = pf.ID
	}
	return p
}

// RecordPattern inserts or replaces a pattern and all its relations.
// Returns the pattern with its assigned ID.
func (s *Store) RecordPattern(ctx context.Context, p IncidentPattern) (IncidentPattern, error) {
	if p.ID == "" {
		p.ID = "PAT-" + uuid.New().String()[:8]
	}
	now := time.Now().Unix()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Status == "" {
		p.Status = "active"
	}

	// Assign IDs to sub-records that need them.
	for i := range p.FailedFixes {
		if p.FailedFixes[i].ID == "" {
			p.FailedFixes[i].ID = uuid.New().String()
		}
		p.FailedFixes[i].PatternID = p.ID
		if p.FailedFixes[i].CreatedAt == 0 {
			p.FailedFixes[i].CreatedAt = now
		}
	}
	for i := range p.EditShapes {
		if p.EditShapes[i].ID == "" {
			p.EditShapes[i].ID = uuid.New().String()
		}
		p.EditShapes[i].PatternID = p.ID
	}
	for i := range p.Files {
		p.Files[i].PatternID = p.ID
	}
	for i := range p.Symbols {
		p.Symbols[i].PatternID = p.ID
	}
	for i := range p.Invariants {
		p.Invariants[i].PatternID = p.ID
	}
	for i := range p.Proposals {
		p.Proposals[i].PatternID = p.ID
	}

	pf := &patternFile{
		ID:          p.ID,
		IncidentID:  p.IncidentID,
		Title:       p.Title,
		Summary:     p.Summary,
		Severity:    p.Severity,
		Status:      p.Status,
		FailureMode: p.FailureMode,
		RootCause:   p.RootCause,
		Lesson:      p.Lesson,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		Files:       p.Files,
		Symbols:     p.Symbols,
		Invariants:  p.Invariants,
		FailedFixes: p.FailedFixes,
		EditShapes:  p.EditShapes,
		Proposals:   p.Proposals,
	}

	if err := s.writePattern(pf); err != nil {
		return p, fmt.Errorf("incidentpattern: insert pattern: %w", err)
	}
	return p, nil
}

// LoadPattern loads a pattern by ID with all relations.
func (s *Store) LoadPattern(ctx context.Context, id string) (*IncidentPattern, error) {
	pf, err := s.readPattern(id)
	if err != nil {
		return nil, err
	}
	return patternFileToIncidentPattern(pf), nil
}

// LoadPatternByIncident loads a pattern by incident ID.
func (s *Store) LoadPatternByIncident(ctx context.Context, incidentID string) (*IncidentPattern, error) {
	patterns, err := s.listAllPatterns()
	if err != nil {
		return nil, err
	}
	var latest *patternFile
	for _, pf := range patterns {
		if pf.IncidentID != incidentID {
			continue
		}
		if latest == nil || pf.CreatedAt > latest.CreatedAt {
			p := *pf
			latest = &p
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("incidentpattern: no pattern for incident %s", incidentID)
	}
	return patternFileToIncidentPattern(latest), nil
}

// ListPatterns returns all active patterns with their relations.
func (s *Store) ListPatterns(ctx context.Context) ([]IncidentPattern, error) {
	patterns, err := s.listAllPatterns()
	if err != nil {
		return nil, fmt.Errorf("incidentpattern: list: %w", err)
	}

	// Sort by created_at descending.
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].CreatedAt > patterns[j].CreatedAt
	})

	var out []IncidentPattern
	for _, pf := range patterns {
		if pf.Status != "active" {
			continue
		}
		out = append(out, *patternFileToIncidentPattern(pf))
	}
	return out, nil
}

// listAllPatterns reads all patterns (from memory or files).
func (s *Store) listAllPatterns() ([]*patternFile, error) {
	if s.dataDir == "" {
		// In-memory mode — use shared graph-level storage.
		s.shared.mu.Lock()
		defer s.shared.mu.Unlock()
		var out []*patternFile
		for _, pf := range s.shared.patterns {
			cp := *pf
			out = append(out, &cp)
		}
		return out, nil
	}

	dir := s.patternsDir()
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
	var out []*patternFile
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var pf patternFile
		if err := json.Unmarshal(data, &pf); err != nil {
			continue
		}
		p := pf
		out = append(out, &p)
	}
	return out, nil
}

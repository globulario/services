package failuregraph

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

// Store provides persistence for the failure knowledge graph backed by JSON files
// or in-memory maps when the graph has no data directory.
type Store struct {
	mu      sync.Mutex
	dataDir string // base data directory from graph; "" = in-memory

	// In-memory maps used when dataDir == "".
	memNodes    map[string]*FailureNode
	memEdges    map[string]*FailureEdge
	memSigs     map[string]*ErrorSignature
	memObs      map[string]*FailureObservation
	memRecipes  map[string]*ResolutionRecipe
	memWFModes  map[string]*WorkflowFailureMode
}

// New returns a Store backed by the given awareness graph.
func New(g *graph.Graph) *Store {
	return &Store{
		dataDir:    g.DataDir(),
		memNodes:   make(map[string]*FailureNode),
		memEdges:   make(map[string]*FailureEdge),
		memSigs:    make(map[string]*ErrorSignature),
		memObs:     make(map[string]*FailureObservation),
		memRecipes: make(map[string]*ResolutionRecipe),
		memWFModes: make(map[string]*WorkflowFailureMode),
	}
}

// subdirFor returns the JSON persistence directory for a given record type.
func (s *Store) subdirFor(kind string) string {
	if s.dataDir == "" {
		return ""
	}
	d := filepath.Join(s.dataDir, "failure_graph", kind)
	_ = os.MkdirAll(d, 0o755)
	return d
}

// sanitizeFileID converts an id to a filesystem-safe filename component.
func sanitizeFileID(id string) string {
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

func (s *Store) writeRecord(kind, id string, v any) error {
	dir := s.subdirFor(kind)
	if dir == "" {
		return nil // in-memory only
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONAtomic(filepath.Join(dir, sanitizeFileID(id)+".json"), v)
}

func (s *Store) readRecord(kind, id string, v any) error {
	dir := s.subdirFor(kind)
	if dir == "" {
		return fmt.Errorf("failuregraph: in-memory graph, no record %s/%s", kind, id)
	}
	data, err := os.ReadFile(filepath.Join(dir, sanitizeFileID(id)+".json"))
	if err != nil {
		return fmt.Errorf("failuregraph: load %s/%s: %w", kind, id, err)
	}
	return json.Unmarshal(data, v)
}

func (s *Store) listRecords(kind string) ([][]byte, error) {
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

// RecordFailureNode inserts or updates a failure node.
func (s *Store) RecordFailureNode(ctx context.Context, n FailureNode) (*FailureNode, error) {
	if n.ID == "" {
		n.ID = nodePrefix(n.NodeType) + uuid.New().String()[:8]
	}
	now := time.Now().Unix()
	if n.CreatedAt == 0 {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	if n.Status == "" {
		n.Status = StatusActive
	}
	if s.dataDir == "" {
		s.mu.Lock()
		cp := n
		s.memNodes[n.ID] = &cp
		s.mu.Unlock()
		return &n, nil
	}
	if err := s.writeRecord("nodes", n.ID, &n); err != nil {
		return nil, fmt.Errorf("failuregraph: insert node %s: %w", n.ID, err)
	}
	return &n, nil
}

// RecordFailureEdge inserts a directed typed edge between two nodes.
func (s *Store) RecordFailureEdge(ctx context.Context, e FailureEdge) (*FailureEdge, error) {
	if e.ID == "" {
		e.ID = "FEDGE-" + uuid.New().String()[:8]
	}
	e.CreatedAt = time.Now().Unix()
	if s.dataDir == "" {
		s.mu.Lock()
		cp := e
		s.memEdges[e.ID] = &cp
		s.mu.Unlock()
		return &e, nil
	}
	if err := s.writeRecord("edges", e.ID, &e); err != nil {
		return nil, fmt.Errorf("failuregraph: insert edge %s->%s (%s): %w", e.FromID, e.ToID, e.EdgeType, err)
	}
	return &e, nil
}

// RecordErrorSignature inserts or updates a normalized error signature.
func (s *Store) RecordErrorSignature(ctx context.Context, sig ErrorSignature) (*ErrorSignature, error) {
	if sig.ID == "" {
		sig.ID = "ERRSIG-" + uuid.New().String()[:8]
	}
	now := time.Now().Unix()
	if sig.CreatedAt == 0 {
		sig.CreatedAt = now
	}
	sig.UpdatedAt = now
	if sig.MatcherKind == "" {
		sig.MatcherKind = MatcherKindExact
	}
	if s.dataDir == "" {
		s.mu.Lock()
		cp := sig
		s.memSigs[sig.ID] = &cp
		s.mu.Unlock()
		return &sig, nil
	}
	if err := s.writeRecord("signatures", sig.ID, &sig); err != nil {
		return nil, fmt.Errorf("failuregraph: insert signature %s: %w", sig.ID, err)
	}
	return &sig, nil
}

// RecordObservation inserts a failure observation record.
func (s *Store) RecordObservation(ctx context.Context, obs FailureObservation) (*FailureObservation, error) {
	if obs.ID == "" {
		obs.ID = "FOBS-" + uuid.New().String()[:8]
	}
	obs.CreatedAt = time.Now().Unix()
	if s.dataDir == "" {
		s.mu.Lock()
		cp := obs
		s.memObs[obs.ID] = &cp
		s.mu.Unlock()
		return &obs, nil
	}
	if err := s.writeRecord("observations", obs.ID, &obs); err != nil {
		return nil, fmt.Errorf("failuregraph: insert observation: %w", err)
	}
	return &obs, nil
}

// RecordResolutionRecipe inserts or updates a resolution recipe.
func (s *Store) RecordResolutionRecipe(ctx context.Context, r ResolutionRecipe) (*ResolutionRecipe, error) {
	if r.ID == "" {
		r.ID = "RECIPE-" + uuid.New().String()[:8]
	}
	now := time.Now().Unix()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if s.dataDir == "" {
		s.mu.Lock()
		cp := r
		s.memRecipes[r.ID] = &cp
		s.mu.Unlock()
		return &r, nil
	}
	if err := s.writeRecord("recipes", r.ID, &r); err != nil {
		return nil, fmt.Errorf("failuregraph: insert recipe %s: %w", r.ID, err)
	}
	return &r, nil
}

// RecordWorkflowFailureMode inserts or updates a workflow failure mode.
func (s *Store) RecordWorkflowFailureMode(ctx context.Context, m WorkflowFailureMode) (*WorkflowFailureMode, error) {
	if m.ID == "" {
		m.ID = "WFMODE-" + uuid.New().String()[:8]
	}
	now := time.Now().Unix()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if s.dataDir == "" {
		s.mu.Lock()
		cp := m
		s.memWFModes[m.ID] = &cp
		s.mu.Unlock()
		return &m, nil
	}
	if err := s.writeRecord("workflow_modes", m.ID, &m); err != nil {
		return nil, fmt.Errorf("failuregraph: insert workflow mode %s: %w", m.ID, err)
	}
	return &m, nil
}

// LoadNode loads a failure node by ID.
func (s *Store) LoadNode(ctx context.Context, id string) (*FailureNode, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		n, ok := s.memNodes[id]
		s.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("failuregraph: in-memory graph, no node %s", id)
		}
		cp := *n
		return &cp, nil
	}
	var n FailureNode
	if err := s.readRecord("nodes", id, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// ListCategories returns all active ErrorCategory nodes.
func (s *Store) ListCategories(ctx context.Context) ([]FailureNode, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		var nodes []FailureNode
		for _, n := range s.memNodes {
			if n.NodeType == NodeTypeErrorCategory && n.Status == StatusActive {
				nodes = append(nodes, *n)
			}
		}
		return nodes, nil
	}
	blobs, err := s.listRecords("nodes")
	if err != nil {
		return nil, fmt.Errorf("failuregraph: list categories: %w", err)
	}
	var nodes []FailureNode
	for _, data := range blobs {
		var n FailureNode
		if err := json.Unmarshal(data, &n); err != nil {
			continue
		}
		if n.NodeType == NodeTypeErrorCategory && n.Status == StatusActive {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// NodesReachable returns all nodes reachable from fromID via the given edge type.
func (s *Store) NodesReachable(ctx context.Context, fromID, edgeType string) ([]FailureNode, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		targetSet := make(map[string]bool)
		for _, e := range s.memEdges {
			if e.FromID == fromID && e.EdgeType == edgeType {
				targetSet[e.ToID] = true
			}
		}
		if len(targetSet) == 0 {
			return nil, nil
		}
		var nodes []FailureNode
		for _, n := range s.memNodes {
			if targetSet[n.ID] && n.Status == StatusActive {
				nodes = append(nodes, *n)
			}
		}
		return nodes, nil
	}

	// First find all edges from fromID with edgeType.
	edgeBlobs, err := s.listRecords("edges")
	if err != nil {
		return nil, fmt.Errorf("failuregraph: reachable %s-[%s]->?: %w", fromID, edgeType, err)
	}
	var targetIDs []string
	for _, data := range edgeBlobs {
		var e FailureEdge
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		if e.FromID == fromID && e.EdgeType == edgeType {
			targetIDs = append(targetIDs, e.ToID)
		}
	}

	if len(targetIDs) == 0 {
		return nil, nil
	}

	// Build a set of target IDs for O(1) lookup.
	targetSet := make(map[string]bool, len(targetIDs))
	for _, id := range targetIDs {
		targetSet[id] = true
	}

	// Load matching nodes.
	nodeBlobs, err := s.listRecords("nodes")
	if err != nil {
		return nil, fmt.Errorf("failuregraph: reachable nodes: %w", err)
	}
	var nodes []FailureNode
	for _, data := range nodeBlobs {
		var n FailureNode
		if err := json.Unmarshal(data, &n); err != nil {
			continue
		}
		if targetSet[n.ID] && n.Status == StatusActive {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// LoadWorkflowModes returns all WorkflowFailureMode rows linked to a category
// via EdgeCommonlyCausedBy edges.
func (s *Store) LoadWorkflowModes(ctx context.Context, categoryID string) ([]WorkflowFailureMode, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		targetSet := make(map[string]bool)
		for _, e := range s.memEdges {
			if e.FromID == categoryID && e.EdgeType == EdgeCommonlyCausedBy {
				targetSet[e.ToID] = true
			}
		}
		if len(targetSet) == 0 {
			return nil, nil
		}
		var modes []WorkflowFailureMode
		for _, m := range s.memWFModes {
			if targetSet[m.ID] {
				modes = append(modes, *m)
			}
		}
		return modes, nil
	}

	// Find edges from categoryID with EdgeCommonlyCausedBy.
	edgeBlobs, err := s.listRecords("edges")
	if err != nil {
		return nil, fmt.Errorf("failuregraph: workflow modes for %s: %w", categoryID, err)
	}
	var targetIDs []string
	for _, data := range edgeBlobs {
		var e FailureEdge
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		if e.FromID == categoryID && e.EdgeType == EdgeCommonlyCausedBy {
			targetIDs = append(targetIDs, e.ToID)
		}
	}

	if len(targetIDs) == 0 {
		return nil, nil
	}

	targetSet := make(map[string]bool, len(targetIDs))
	for _, id := range targetIDs {
		targetSet[id] = true
	}

	modeBlobs, err := s.listRecords("workflow_modes")
	if err != nil {
		return nil, fmt.Errorf("failuregraph: load workflow modes: %w", err)
	}
	var modes []WorkflowFailureMode
	for _, data := range modeBlobs {
		var m WorkflowFailureMode
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		if targetSet[m.ID] {
			modes = append(modes, m)
		}
	}
	return modes, nil
}

// AllSignatures returns all active error signatures.
func (s *Store) AllSignatures(ctx context.Context) ([]ErrorSignature, error) {
	if s.dataDir == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		var sigs []ErrorSignature
		for _, sig := range s.memSigs {
			sigs = append(sigs, *sig)
		}
		return sigs, nil
	}
	blobs, err := s.listRecords("signatures")
	if err != nil {
		return nil, fmt.Errorf("failuregraph: all signatures: %w", err)
	}
	var sigs []ErrorSignature
	for _, data := range blobs {
		var sig ErrorSignature
		if err := json.Unmarshal(data, &sig); err != nil {
			continue
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

// nodePrefix returns the ID prefix for a node type.
func nodePrefix(nodeType string) string {
	switch nodeType {
	case NodeTypeErrorCategory:
		return "ERRCAT-"
	case NodeTypeSymptom:
		return "SYM-"
	case NodeTypeRootCause:
		return "CAUSE-"
	case NodeTypeResolution:
		return "RES-"
	case NodeTypeWrongFix:
		return "WRONG-"
	case NodeTypeRegressionTest:
		return "REGTEST-"
	case NodeTypeInvariant:
		return "INV-"
	case NodeTypeWorkflowMode:
		return "WFMODE-"
	case NodeTypeIncidentExample:
		return "INCEX-"
	case NodeTypeSemanticAtom:
		return "SATOM-"
	case NodeTypeRuntimeSignal:
		return "RTSIG-"
	default:
		return "FN-"
	}
}

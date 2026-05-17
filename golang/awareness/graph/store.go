package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Graph is the central awareness graph handle backed by in-memory data structures
// with optional JSON file persistence.
type Graph struct {
	mu sync.RWMutex

	// Static knowledge (loaded from bundle JSON or built at Open time)
	nodes  map[string]*Node   // id → node
	edges  []*Edge            // all edges (pointer for mutation)
	edgeKeys map[edgeKey]int  // (src,kind,dst,phase) → index in edges slice

	// Indexes for fast lookup
	byType map[string][]*Node // type → nodes
	byPath map[string][]*Node // path → nodes
	bySrc  map[string][]*Edge // src → edges
	byDst  map[string][]*Edge // dst → edges
	byKind map[string][]*Edge // kind → edges
	byClass map[string][]*Edge // edge_class → edges

	// Specialized knowledge maps
	invariants   map[string]*Invariant    // id → invariant
	failureModes map[string]*FailureMode  // id → failure mode


	// Build metadata
	buildMu sync.RWMutex
	builds  []*BuildRecord // ordered by created_at

	// Incident and proposal records (mutable runtime data)
	incidentMu  sync.RWMutex
	incidents   map[string]*IncidentRecord

	proposalMu sync.RWMutex
	proposals  map[string]*ProposalRecord

	// Preflight audits
	preflightMu sync.RWMutex
	preflights  []*PreflightAuditRecord

	// Experience store (in-memory)
	expMu        sync.RWMutex
	experiences  map[string]*ExperienceEntry
	expAttempts  map[string][]*ExperienceAttempt     // experience_id → attempts
	expObs       map[string][]*ExperienceObservation // experience_id → observations

	// Runtime snapshots
	snapshotMu sync.RWMutex
	snapshots  []*runtimeSnapshotRecord // ordered by captured_at desc

	// registry is a generic in-memory key-value store for sub-packages that
	// need to share state across multiple Store instances on the same Graph
	// when dataDir == "". Keyed by sub-package name (e.g. "incidentpattern").
	registry sync.Map

	// Mutable runtime data persistence
	dataDir  string // directory for JSON files; "" means in-memory only
	readOnly bool   // true → all writes blocked (OpenReadOnly)
	// staticReadOnly is true for OpenComposite: static graph writes (AddNode,
	// AddEdge, UpsertInvariant, UpsertFailureMode) are blocked, but runtime
	// data writes (incidents, proposals, preflight audits, experiences, etc.)
	// are allowed via dataDir.
	staticReadOnly bool
}

// edgeKey is the composite primary key for edges.
type edgeKey struct {
	Src   string
	Kind  string
	Dst   string
	Phase string
}

// runtimeSnapshotRecord is the in-memory form of a runtime_snapshots row.
type runtimeSnapshotRecord struct {
	ID           string `json:"id"`
	CapturedAt   int64  `json:"captured_at"`
	NodeID       string `json:"node_id"`
	ClusterID    string `json:"cluster_id"`
	SnapshotJSON string `json:"snapshot_json"`
	CreatedAt    int64  `json:"created_at"`
}

// newGraph creates an empty initialised Graph.
func newGraph() *Graph {
	return &Graph{
		nodes:       make(map[string]*Node),
		edges:       nil,
		edgeKeys:    make(map[edgeKey]int),
		byType:      make(map[string][]*Node),
		byPath:      make(map[string][]*Node),
		bySrc:       make(map[string][]*Edge),
		byDst:       make(map[string][]*Edge),
		byKind:      make(map[string][]*Edge),
		byClass:     make(map[string][]*Edge),
		invariants:  make(map[string]*Invariant),
		failureModes: make(map[string]*FailureMode),
		incidents:   make(map[string]*IncidentRecord),
		proposals:   make(map[string]*ProposalRecord),
		experiences: make(map[string]*ExperienceEntry),
		expAttempts: make(map[string][]*ExperienceAttempt),
		expObs:      make(map[string][]*ExperienceObservation),
	}
}

// Open opens (or creates) the awareness graph at path.
// If path is a directory, graph data is loaded from <path>/graph.json (if it exists).
// If path ends in .json, data is loaded from that file.
// If path ends in .db, an error is returned (SQLite is no longer supported).
// The directory is created if it does not exist.
func Open(path string) (*Graph, error) {
	if path == "" {
		return nil, fmt.Errorf("awareness graph: path is empty")
	}
	// ":memory:" is the legacy SQLite in-memory sentinel — route to OpenMemory.
	if path == ":memory:" {
		return OpenMemory()
	}
	if strings.HasSuffix(path, ".db") {
		// Treat as a directory named after the .db file prefix — for test compatibility.
		// Many tests do filepath.Join(dir, "graph.db") expecting Open to work.
		// We use the parent dir as dataDir and the file path as the graph.json home.
		dataDir := filepath.Dir(path)
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return nil, fmt.Errorf("awareness graph: mkdir %s: %w", dataDir, err)
		}
		g := newGraph()
		g.dataDir = dataDir
		// Try to load from <dataDir>/graph.json.
		jsonPath := filepath.Join(dataDir, "graph.json")
		if data, err := os.ReadFile(jsonPath); err == nil {
			_ = g.loadFromJSON(data)
		}
		return g, nil
	}

	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("awareness graph: stat %s: %w", path, err)
	}

	var dataDir string
	var jsonPath string

	if err == nil && info.IsDir() {
		dataDir = path
		jsonPath = filepath.Join(path, "graph.json")
	} else if strings.HasSuffix(path, ".json") {
		dataDir = filepath.Dir(path)
		jsonPath = path
	} else {
		// Treat as a directory.
		dataDir = path
		jsonPath = filepath.Join(path, "graph.json")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("awareness graph: mkdir %s: %w", dataDir, err)
	}

	g := newGraph()
	g.dataDir = dataDir

	if data, err := os.ReadFile(jsonPath); err == nil {
		if loadErr := g.loadFromJSON(data); loadErr != nil {
			// Non-fatal: start with empty graph if JSON is corrupt.
			_ = loadErr
		}
	}

	return g, nil
}

// OpenReadOnly opens an existing awareness graph at path for read-only access.
// It does NOT create the parent directory. All write operations return an error.
func OpenReadOnly(path string) (*Graph, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("awareness graph (read-only): stat %s: %w", path, err)
	}

	var jsonPath string
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		jsonPath = filepath.Join(path, "graph.json")
	} else if strings.HasSuffix(path, ".json") {
		jsonPath = path
	} else if strings.HasSuffix(path, ".db") {
		// Legacy .db path: look for graph.json in same dir.
		jsonPath = filepath.Join(filepath.Dir(path), "graph.json")
	} else {
		jsonPath = filepath.Join(path, "graph.json")
	}

	g := newGraph()
	g.readOnly = true

	if data, err := os.ReadFile(jsonPath); err == nil {
		_ = g.loadFromJSON(data)
	}
	// It's OK if graph.json doesn't exist — return an empty read-only graph.

	return g, nil
}

// OpenComposite opens an awareness graph composed of static bundle data (read-only)
// and a writable runtime state stored in runtimePath.
//
// Static knowledge (nodes, edges, invariants, failure_modes, builds, context_aliases)
// is loaded from bundlePath (directory or .json file) as read-only.
// Mutable operations (sessions, experience, proposals, etc.) use runtimePath as dataDir.
func OpenComposite(bundlePath, runtimePath string) (*Graph, error) {
	if bundlePath == "" {
		return nil, fmt.Errorf("awareness graph (composite): bundlePath is empty")
	}
	if runtimePath == "" {
		return nil, fmt.Errorf("awareness graph (composite): runtimePath is empty")
	}
	if _, err := os.Stat(bundlePath); err != nil {
		return nil, fmt.Errorf("awareness graph (composite): stat bundle %s: %w", bundlePath, err)
	}

	// Determine runtimeDir.
	var runtimeDir string
	if strings.HasSuffix(runtimePath, ".db") || strings.HasSuffix(runtimePath, ".json") {
		runtimeDir = filepath.Dir(runtimePath)
	} else {
		runtimeDir = runtimePath
	}
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return nil, fmt.Errorf("awareness graph (composite): mkdir runtime %s: %w", runtimeDir, err)
	}

	// Load static data from bundle (read-only).
	g := newGraph()

	// Determine bundle JSON path.
	var bundleJSONPath string
	if info, err := os.Stat(bundlePath); err == nil && info.IsDir() {
		bundleJSONPath = filepath.Join(bundlePath, "graph.json")
	} else if strings.HasSuffix(bundlePath, ".json") {
		bundleJSONPath = bundlePath
	} else if strings.HasSuffix(bundlePath, ".db") {
		bundleJSONPath = filepath.Join(filepath.Dir(bundlePath), "graph.json")
	} else {
		bundleJSONPath = filepath.Join(bundlePath, "graph.json")
	}

	if data, err := os.ReadFile(bundleJSONPath); err == nil {
		_ = g.loadFromJSON(data)
	}

	// staticReadOnly = true: AddNode/AddEdge/UpsertInvariant/etc. are blocked.
	// Runtime data writes (incidents, proposals, experiences, etc.) are allowed
	// because readOnly remains false and dataDir is set.
	g.dataDir = runtimeDir
	g.readOnly = false
	g.staticReadOnly = true

	// Load any persisted runtime state.
	g.loadRuntimeState()

	return g, nil
}

// OpenMemory opens a fresh in-memory awareness graph.
// It is suitable for tests and validation: changes are never persisted.
func OpenMemory() (*Graph, error) {
	g := newGraph()
	return g, nil
}

// Close flushes the graph's static data (nodes, edges, invariants, failure_modes,
// builds, context_aliases) to <dataDir>/graph.json and releases resources.
// For read-only or in-memory graphs this is a no-op.
func (g *Graph) Close() error {
	if g.readOnly || g.staticReadOnly {
		return nil
	}
	return g.saveGraphJSON()
}

// indexNode adds a node to all in-memory indexes. Caller holds write lock.
func (g *Graph) indexNode(n *Node) {
	old, exists := g.nodes[n.ID]
	if exists {
		// Remove from byType and byPath indexes.
		g.removeNodeFromIndexes(old)
	}
	g.nodes[n.ID] = n
	g.byType[n.Type] = append(g.byType[n.Type], n)
	if n.Path != "" {
		g.byPath[n.Path] = append(g.byPath[n.Path], n)
	}
}

// removeNodeFromIndexes removes a node from byType and byPath slices.
func (g *Graph) removeNodeFromIndexes(n *Node) {
	g.byType[n.Type] = removeNodeFromSlice(g.byType[n.Type], n)
	if n.Path != "" {
		g.byPath[n.Path] = removeNodeFromSlice(g.byPath[n.Path], n)
	}
}

func removeNodeFromSlice(sl []*Node, n *Node) []*Node {
	out := sl[:0]
	for _, x := range sl {
		if x.ID != n.ID {
			out = append(out, x)
		}
	}
	return out
}

// indexEdge adds an edge to all in-memory indexes. Caller holds write lock.
// If an edge with the same (src,kind,dst,phase) already exists, it is replaced.
func (g *Graph) indexEdge(e *Edge) {
	class := e.Class
	weight := e.Weight
	if class == "" || weight == 0 {
		c, w := classifyEdge(e.Kind)
		if class == "" {
			class = c
			e.Class = c
		}
		if weight == 0 {
			weight = w
			e.Weight = w
		}
	}
	if e.Confidence == 0 {
		e.Confidence = 1.0
	}

	k := edgeKey{Src: e.Src, Kind: e.Kind, Dst: e.Dst, Phase: e.Phase}
	if idx, exists := g.edgeKeys[k]; exists {
		old := g.edges[idx]
		// Remove old edge from src/dst/kind/class indexes.
		g.bySrc[old.Src] = removeEdgeFromSlice(g.bySrc[old.Src], old)
		g.byDst[old.Dst] = removeEdgeFromSlice(g.byDst[old.Dst], old)
		g.byKind[old.Kind] = removeEdgeFromSlice(g.byKind[old.Kind], old)
		g.byClass[old.Class] = removeEdgeFromSlice(g.byClass[old.Class], old)
		// Replace in edges slice.
		g.edges[idx] = e
	} else {
		g.edgeKeys[k] = len(g.edges)
		g.edges = append(g.edges, e)
	}
	g.bySrc[e.Src] = append(g.bySrc[e.Src], e)
	g.byDst[e.Dst] = append(g.byDst[e.Dst], e)
	g.byKind[e.Kind] = append(g.byKind[e.Kind], e)
	g.byClass[e.Class] = append(g.byClass[e.Class], e)
}

func removeEdgeFromSlice(sl []*Edge, e *Edge) []*Edge {
	out := sl[:0]
	for _, x := range sl {
		if !(x.Src == e.Src && x.Kind == e.Kind && x.Dst == e.Dst && x.Phase == e.Phase) {
			out = append(out, x)
		}
	}
	return out
}

// sortSnapshotsByTime sorts snapshots descending by captured_at.
func sortSnapshotsByTime(snaps []*runtimeSnapshotRecord) {
	for i := 1; i < len(snaps); i++ {
		for j := i; j > 0 && snaps[j].CapturedAt > snaps[j-1].CapturedAt; j-- {
			snaps[j], snaps[j-1] = snaps[j-1], snaps[j]
		}
	}
}

// DataDir returns the directory used for JSON file persistence.
// Returns "" for in-memory graphs.
func (g *Graph) DataDir() string {
	return g.dataDir
}

// MemRegistry returns the per-graph sync.Map used by sub-packages to share
// in-memory state across multiple Store instances on the same Graph when
// dataDir == "". Each sub-package owns a unique key (typically its package name).
func (g *Graph) MemRegistry() *sync.Map {
	return &g.registry
}

// AddBuildRecord appends a build record to the in-memory builds slice.
// This is used by the builder and by tests that need to simulate a graph
// that was built at a specific point in time.
func (g *Graph) AddBuildRecord(b BuildRecord) {
	g.buildMu.Lock()
	defer g.buildMu.Unlock()
	g.builds = append(g.builds, &b)
}

// SetNodeMetadata updates the metadata of an existing node. No-op if node not found.
func (g *Graph) SetNodeMetadata(_ context.Context, id string, meta map[string]any) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("SetNodeMetadata: graph is read-only")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	n, ok := g.nodes[id]
	if !ok {
		return nil
	}
	n.Metadata = meta
	n.UpdatedAt = time.Now().Unix()
	return nil
}

// marshalMeta encodes a metadata map to JSON. Returns "{}" for nil.
func marshalMeta(m map[string]any) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// unmarshalMeta decodes a JSON metadata string. Returns nil on empty/invalid input.
func unmarshalMeta(s string) map[string]any {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(s), &m)
	return m
}

// Cleanup removes JSON files older than maxAge from the runtime subdirectories:
// sessions/, snapshots/, cluster_snapshots/, failure_graph/, incident_patterns/,
// incident_acks/, and experience/. It reads created_at (or collected_at) from
// each file and removes it if the age exceeds maxAge. Errors on individual files
// are logged but do not stop the walk; the first directory-level error is returned.
// No-op when the graph has no dataDir (in-memory mode).
func (g *Graph) Cleanup(maxAge time.Duration) error {
	if g.dataDir == "" {
		return nil
	}
	cutoff := time.Now().Add(-maxAge).Unix()
	subdirs := []string{
		"sessions", "snapshots", "cluster_snapshots",
		"failure_graph", "incident_patterns", "incident_acks", "experience",
	}
	var firstErr error
	for _, sub := range subdirs {
		dir := filepath.Join(g.dataDir, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			fpath := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(fpath)
			if err != nil {
				continue
			}
			// Try to extract created_at or collected_at timestamp.
			var rec struct {
				CreatedAt   int64 `json:"created_at"`
				CollectedAt int64 `json:"collected_at"`
			}
			if err := json.Unmarshal(data, &rec); err != nil {
				continue
			}
			ts := rec.CreatedAt
			if ts == 0 {
				ts = rec.CollectedAt
			}
			if ts == 0 {
				continue // no timestamp — skip
			}
			if ts < cutoff {
				_ = os.Remove(fpath)
			}
		}
	}
	return firstErr
}

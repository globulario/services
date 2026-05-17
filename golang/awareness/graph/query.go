package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Invariant is the specialized invariant record.
type Invariant struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	Severity string `json:"severity,omitempty"`
	Status   string `json:"status,omitempty"`
}

// FailureMode is the specialized failure mode record.
type FailureMode struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary,omitempty"`
	Symptoms        []string `json:"symptoms,omitempty"`
	RootCause       string   `json:"root_cause,omitempty"`
	ArchitectureFix string   `json:"architecture_fix,omitempty"`
}

// BuildStats holds graph build statistics.
type BuildStats struct {
	Nodes                 int   `json:"nodes"`
	Edges                 int   `json:"edges"`
	Invariants            int   `json:"invariants"`
	FailureModes          int   `json:"failure_modes"`
	FilesScanned          int   `json:"files_scanned,omitempty"`
	KnowledgeFilesScanned int   `json:"knowledge_files_scanned,omitempty"`
	DurationMs            int64 `json:"duration_ms,omitempty"`
}

// CollectorHealthItem records the outcome of a single collector pass.
type CollectorHealthItem struct {
	CollectorID  string `json:"collector_id"`
	SourceTier   string `json:"source_tier,omitempty"`
	Status       string `json:"status"`
	NodesEmitted int    `json:"nodes_emitted"`
	Error        string `json:"error,omitempty"`
	Priority     string `json:"priority,omitempty"`
}

// BuildRecord is a single graph build record.
type BuildRecord struct {
	ID              string               `json:"id"`
	RepoRoot        string               `json:"repo_root"`
	GitCommit       string               `json:"git_commit,omitempty"`
	ReleaseID       string               `json:"release_id,omitempty"`
	CreatedAt       int64                `json:"created_at"`
	Stats           BuildStats           `json:"stats"`
	CollectorHealth []CollectorHealthItem `json:"collector_health,omitempty"`
}

// LatestBuildRecord returns the most recent static graph build row (excludes live snapshots).
func (g *Graph) LatestBuildRecord(ctx context.Context) (*BuildRecord, error) {
	g.buildMu.RLock()
	defer g.buildMu.RUnlock()
	var latest *BuildRecord
	for _, b := range g.builds {
		if b.ID == LiveSnapshotBuildID {
			continue
		}
		if latest == nil || b.CreatedAt > latest.CreatedAt {
			latest = b
		}
	}
	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &cp, nil
}

// LiveSnapshotBuildID is the fixed build ID used for live overlay refresh records.
const LiveSnapshotBuildID = "live-snapshot"

// LatestLiveSnapshotRecord returns the most recent live mirror refresh record.
func (g *Graph) LatestLiveSnapshotRecord(ctx context.Context) (*BuildRecord, error) {
	g.buildMu.RLock()
	defer g.buildMu.RUnlock()
	for _, b := range g.builds {
		if b.ID == LiveSnapshotBuildID {
			cp := *b
			return &cp, nil
		}
	}
	return nil, nil
}

// SetBuildCollectorHealth stores the collector health array for a build record.
func (g *Graph) SetBuildCollectorHealth(ctx context.Context, buildID string, items []CollectorHealthItem) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("SetBuildCollectorHealth: graph is read-only")
	}
	g.buildMu.Lock()
	defer g.buildMu.Unlock()
	for _, b := range g.builds {
		if b.ID == buildID {
			b.CollectorHealth = items
			return nil
		}
	}
	return nil // build not found — no-op
}

// FindNode returns a node by ID, or (nil, nil) if not found.
func (g *Graph) FindNode(ctx context.Context, id string) (*Node, error) {
	g.mu.RLock()
	n := g.nodes[id]
	g.mu.RUnlock()
	if n == nil {
		return nil, nil
	}
	cp := *n
	return &cp, nil
}

// FindNodesByType returns all nodes of the given type ordered by name.
func (g *Graph) FindNodesByType(ctx context.Context, nodeType string) ([]*Node, error) {
	g.mu.RLock()
	src := g.byType[nodeType]
	out := make([]*Node, len(src))
	for i, n := range src {
		cp := *n
		out[i] = &cp
	}
	g.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// FindNodesByPath returns nodes whose path exactly matches the given value.
func (g *Graph) FindNodesByPath(ctx context.Context, path string) ([]*Node, error) {
	g.mu.RLock()
	src := g.byPath[path]
	out := make([]*Node, len(src))
	for i, n := range src {
		cp := *n
		out[i] = &cp
	}
	g.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// FindNodeByTypeAndName returns the first node matching type + exact name.
func (g *Graph) FindNodeByTypeAndName(ctx context.Context, nodeType, name string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, n := range g.byType[nodeType] {
		if n.Name == name {
			cp := *n
			return &cp, nil
		}
	}
	return nil, nil
}

// FindNodesByNameLike returns nodes whose name contains the query string (case-insensitive).
func (g *Graph) FindNodesByNameLike(ctx context.Context, query string) ([]*Node, error) {
	q := strings.ToLower(query)
	g.mu.RLock()
	var out []*Node
	for _, n := range g.nodes {
		if strings.Contains(strings.ToLower(n.Name), q) {
			cp := *n
			out = append(out, &cp)
		}
	}
	g.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Neighbors returns edges connected to id.
// direction is "out" (outgoing), "in" (incoming), or "both".
func (g *Graph) Neighbors(ctx context.Context, id, direction string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []Edge
	switch direction {
	case "in":
		for _, e := range g.byDst[id] {
			out = append(out, *e)
		}
	case "out":
		for _, e := range g.bySrc[id] {
			out = append(out, *e)
		}
	default:
		seen := make(map[edgeKey]bool)
		for _, e := range g.bySrc[id] {
			k := edgeKey{e.Src, e.Kind, e.Dst, e.Phase}
			if !seen[k] {
				seen[k] = true
				out = append(out, *e)
			}
		}
		for _, e := range g.byDst[id] {
			k := edgeKey{e.Src, e.Kind, e.Dst, e.Phase}
			if !seen[k] {
				seen[k] = true
				out = append(out, *e)
			}
		}
	}
	return out, nil
}

// NeighborsByClass returns outgoing edges from id filtered by edge_class.
func (g *Graph) NeighborsByClass(ctx context.Context, id, class string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []Edge
	for _, e := range g.bySrc[id] {
		if e.Class == class {
			out = append(out, *e)
		}
	}
	return out, nil
}

// EdgesByClass returns all edges in the graph with the given edge_class.
func (g *Graph) EdgesByClass(ctx context.Context, class string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	src := g.byClass[class]
	out := make([]Edge, len(src))
	for i, e := range src {
		out[i] = *e
	}
	return out, nil
}

// TraverseDecision performs BFS from startID following only decision-class edges.
func (g *Graph) TraverseDecision(ctx context.Context, startID string, maxDepth int) (*TraversalResult, error) {
	visited := make(map[string]bool)
	result := &TraversalResult{}

	type item struct {
		id    string
		depth int
	}
	queue := []item{{startID, 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if visited[cur.id] {
			continue
		}
		visited[cur.id] = true

		node, err := g.FindNode(ctx, cur.id)
		if err != nil {
			return nil, err
		}
		if node != nil {
			result.Nodes = append(result.Nodes, node)
		}

		if cur.depth >= maxDepth {
			continue
		}

		edges, err := g.NeighborsByClass(ctx, cur.id, EdgeClassDecision)
		if err != nil {
			return nil, fmt.Errorf("TraverseDecision neighbors %s: %w", cur.id, err)
		}

		for _, e := range edges {
			result.Edges = append(result.Edges, e)
			if !visited[e.Dst] {
				queue = append(queue, item{e.Dst, cur.depth + 1})
			}
		}
	}

	return result, nil
}

// AllEdges returns every edge in the graph (used by cycle detection).
func (g *Graph) AllEdges(ctx context.Context) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Edge, len(g.edges))
	for i, e := range g.edges {
		out[i] = *e
	}
	return out, nil
}

// OutgoingEdges returns all edges where src == nodeID.
func (g *Graph) OutgoingEdges(ctx context.Context, nodeID string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	src := g.bySrc[nodeID]
	out := make([]Edge, len(src))
	for i, e := range src {
		out[i] = *e
	}
	return out, nil
}

// EdgesByKind returns all edges of the given kind.
func (g *Graph) EdgesByKind(ctx context.Context, kind string) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	src := g.byKind[kind]
	out := make([]Edge, len(src))
	for i, e := range src {
		out[i] = *e
	}
	return out, nil
}

// AllInvariants returns all invariant records ordered by ID.
func (g *Graph) AllInvariants(ctx context.Context) ([]*Invariant, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*Invariant, 0, len(g.invariants))
	for _, inv := range g.invariants {
		cp := *inv
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// AllFailureModes returns all failure mode records ordered by ID.
func (g *Graph) AllFailureModes(ctx context.Context) ([]*FailureMode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*FailureMode, 0, len(g.failureModes))
	for _, fm := range g.failureModes {
		cp := *fm
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// UpsertInvariant upserts an invariant record.
func (g *Graph) UpsertInvariant(ctx context.Context, inv Invariant) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertInvariant %s: graph is read-only", inv.ID)
	}
	g.mu.Lock()
	cp := inv
	g.invariants[inv.ID] = &cp
	g.mu.Unlock()
	return nil
}

// UpsertFailureMode upserts a failure mode record.
func (g *Graph) UpsertFailureMode(ctx context.Context, fm FailureMode) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertFailureMode %s: graph is read-only", fm.ID)
	}
	g.mu.Lock()
	cp := fm
	g.failureModes[fm.ID] = &cp
	g.mu.Unlock()
	return nil
}

// Stats returns current node/edge/invariant/failure-mode counts.
func (g *Graph) Stats(ctx context.Context) (BuildStats, error) {
	g.mu.RLock()
	s := BuildStats{
		Nodes:        len(g.nodes),
		Edges:        len(g.edges),
		Invariants:   len(g.invariants),
		FailureModes: len(g.failureModes),
	}
	g.mu.RUnlock()
	return s, nil
}

// UpsertBuildRecord records a completed graph build with its stats.
func (g *Graph) UpsertBuildRecord(ctx context.Context, id, repoRoot, gitCommit, releaseID string, stats BuildStats) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertBuildRecord: graph is read-only")
	}
	now := time.Now().Unix()
	g.buildMu.Lock()
	defer g.buildMu.Unlock()
	for _, b := range g.builds {
		if b.ID == id {
			b.RepoRoot = repoRoot
			b.GitCommit = gitCommit
			b.ReleaseID = releaseID
			b.CreatedAt = now
			b.Stats = stats
			return nil
		}
	}
	g.builds = append(g.builds, &BuildRecord{
		ID:        id,
		RepoRoot:  repoRoot,
		GitCommit: gitCommit,
		ReleaseID: releaseID,
		CreatedAt: now,
		Stats:     stats,
	})
	return nil
}

// UpsertBuildRecordAt is like UpsertBuildRecord but accepts an explicit
// Unix timestamp. Used for testing clock-dependent freshness logic.
func (g *Graph) UpsertBuildRecordAt(ctx context.Context, id, repoRoot, gitCommit, releaseID string, stats BuildStats, createdAt int64) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("UpsertBuildRecordAt: graph is read-only")
	}
	g.buildMu.Lock()
	defer g.buildMu.Unlock()
	for _, b := range g.builds {
		if b.ID == id {
			b.RepoRoot = repoRoot
			b.GitCommit = gitCommit
			b.ReleaseID = releaseID
			b.CreatedAt = createdAt
			b.Stats = stats
			return nil
		}
	}
	g.builds = append(g.builds, &BuildRecord{
		ID:        id,
		RepoRoot:  repoRoot,
		GitCommit: gitCommit,
		ReleaseID: releaseID,
		CreatedAt: createdAt,
		Stats:     stats,
	})
	return nil
}

// ---- incident records ----

// IncidentRecord maps to the incidents store.
type IncidentRecord struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Severity     string `json:"severity,omitempty"`
	Status       string `json:"status,omitempty"`
	StartedAt    int64  `json:"started_at,omitempty"`
	EndedAt      int64  `json:"ended_at,omitempty"`
	Summary      string `json:"summary,omitempty"`
	EvidenceJSON string `json:"evidence_json,omitempty"`
	CreatedAt    int64  `json:"created_at,omitempty"`
	UpdatedAt    int64  `json:"updated_at,omitempty"`
}

// ProposalStatus values for awareness_proposals.
const (
	ProposalStatusDraft       = "DRAFT"
	ProposalStatusValidated   = "VALIDATED"
	ProposalStatusNeedsReview = "NEEDS_REVIEW"
	ProposalStatusApproved    = "APPROVED"
	ProposalStatusRejected    = "REJECTED"
	ProposalStatusPromoted    = "PROMOTED"
	ProposalStatusSuperseded  = "SUPERSEDED"
)

// ProposalRecord maps to the awareness_proposals store.
type ProposalRecord struct {
	ID             string `json:"id"`
	IncidentID     string `json:"incident_id,omitempty"`
	Status         string `json:"status"`
	ProposalYAML   string `json:"proposal_yaml"`
	ValidationJSON string `json:"validation_json,omitempty"`
	CreatedBy      string `json:"created_by,omitempty"`
	CreatedAt      int64  `json:"created_at,omitempty"`
	PromotedAt     int64  `json:"promoted_at,omitempty"`
}

// ContextAliasRecord maps to the context_aliases store.
type ContextAliasRecord struct {
	ID         string  `json:"id"`
	TargetID   string  `json:"target_id"`
	Alias      string  `json:"alias"`
	Confidence float64 `json:"confidence,omitempty"`
	Source     string  `json:"source,omitempty"`
	CreatedAt  int64   `json:"created_at,omitempty"`
}

// UpsertIncident inserts or updates an incident record.
func (g *Graph) UpsertIncident(ctx context.Context, inc IncidentRecord) error {
	if g.readOnly {
		return fmt.Errorf("UpsertIncident %s: graph is read-only", inc.ID)
	}
	now := time.Now().Unix()
	if inc.CreatedAt == 0 {
		inc.CreatedAt = now
	}
	inc.UpdatedAt = now

	g.incidentMu.Lock()
	cp := inc
	g.incidents[inc.ID] = &cp
	g.incidentMu.Unlock()

	return g.writeJSON("incidents", inc.ID, &inc)
}

// FindIncident returns an incident by ID, or (nil, nil) if not found.
func (g *Graph) FindIncident(ctx context.Context, id string) (*IncidentRecord, error) {
	g.incidentMu.RLock()
	rec := g.incidents[id]
	g.incidentMu.RUnlock()
	if rec == nil {
		return nil, nil
	}
	cp := *rec
	return &cp, nil
}

// AllProposals returns all proposal records ordered by created_at descending.
func (g *Graph) AllProposals(ctx context.Context) ([]*ProposalRecord, error) {
	g.proposalMu.RLock()
	out := make([]*ProposalRecord, 0, len(g.proposals))
	for _, p := range g.proposals {
		cp := *p
		out = append(out, &cp)
	}
	g.proposalMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// UpdateProposalStatus sets the status (and promoted_at if PROMOTED) of a proposal.
func (g *Graph) UpdateProposalStatus(ctx context.Context, id, status string) error {
	if g.readOnly {
		return fmt.Errorf("UpdateProposalStatus %s: graph is read-only", id)
	}
	g.proposalMu.Lock()
	p := g.proposals[id]
	if p != nil {
		p.Status = status
		if status == ProposalStatusPromoted {
			p.PromotedAt = time.Now().Unix()
		}
	}
	g.proposalMu.Unlock()

	if p != nil {
		return g.writeJSON("proposals", id, p)
	}
	return nil
}

// FindInvariant returns an invariant record by ID, or (nil, nil) if not found.
func (g *Graph) FindInvariant(ctx context.Context, id string) (*Invariant, error) {
	g.mu.RLock()
	inv := g.invariants[id]
	g.mu.RUnlock()
	if inv == nil {
		return nil, nil
	}
	cp := *inv
	return &cp, nil
}

// ---- code smell helpers ----

// CodeSmellsForInvariants returns all code_smells from pattern nodes that
// have a "requires" edge targeting any of the given invariant node IDs.
func (g *Graph) CodeSmellsForInvariants(ctx context.Context, invariantNodeIDs []string) ([]string, error) {
	if len(invariantNodeIDs) == 0 {
		return nil, nil
	}

	invSet := make(map[string]bool, len(invariantNodeIDs))
	for _, id := range invariantNodeIDs {
		invSet[id] = true
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	seen := map[string]bool{}
	var out []string

	// Find pattern nodes that have a requires edge to one of the invariant IDs.
	for _, e := range g.byKind[EdgeRequires] {
		if !invSet[e.Dst] {
			continue
		}
		n := g.nodes[e.Src]
		if n == nil || n.Type != NodeTypePattern {
			continue
		}
		for _, s := range extractCodeSmellsFromMeta(n.Metadata) {
			if s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// PatternNamesForInvariants returns the names of NodeTypePattern nodes
// that have an EdgeRequires edge to any of the given invariant node IDs.
func (g *Graph) PatternNamesForInvariants(ctx context.Context, invariantNodeIDs []string) ([]string, error) {
	if len(invariantNodeIDs) == 0 {
		return nil, nil
	}

	invSet := make(map[string]bool, len(invariantNodeIDs))
	for _, id := range invariantNodeIDs {
		invSet[id] = true
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	seen := map[string]bool{}
	var out []string
	for _, e := range g.byKind[EdgeRequires] {
		if !invSet[e.Dst] {
			continue
		}
		n := g.nodes[e.Src]
		if n == nil || n.Type != NodeTypePattern {
			continue
		}
		if !seen[n.Name] {
			seen[n.Name] = true
			out = append(out, n.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// DesignContext is the set of design patterns, anti-patterns, and code smells
// linked to a given set of invariant node IDs.
type DesignContext struct {
	DesignPatterns []string
	AntiPatterns   []string
	CodeSmells     []string
}

// DesignContextForInvariants returns design patterns and anti-patterns linked
// to the given invariant node IDs.
func (g *Graph) DesignContextForInvariants(ctx context.Context, invariantNodeIDs []string) (*DesignContext, error) {
	if len(invariantNodeIDs) == 0 {
		return &DesignContext{}, nil
	}

	invSet := make(map[string]bool, len(invariantNodeIDs))
	for _, id := range invariantNodeIDs {
		invSet[id] = true
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	dc := &DesignContext{}
	seen := map[string]bool{}
	var antiPatternIDs []string

	// Design patterns: EdgeRequires or EdgeMitigates from design_pattern → invariant.
	for _, kind := range []string{EdgeRequires, EdgeMitigates} {
		for _, e := range g.byKind[kind] {
			if !invSet[e.Dst] {
				continue
			}
			n := g.nodes[e.Src]
			if n == nil || n.Type != NodeTypeDesignPattern {
				continue
			}
			if !seen[n.Name] {
				seen[n.Name] = true
				dc.DesignPatterns = append(dc.DesignPatterns, n.Name)
			}
		}
	}

	// Anti-patterns: EdgeViolates from anti_pattern → invariant.
	for _, e := range g.byKind[EdgeViolates] {
		if !invSet[e.Dst] {
			continue
		}
		n := g.nodes[e.Src]
		if n == nil || n.Type != NodeTypeAntiPattern {
			continue
		}
		if !seen[n.Name] {
			seen[n.Name] = true
			dc.AntiPatterns = append(dc.AntiPatterns, n.Name)
		}
		antiPatternIDs = append(antiPatternIDs, n.ID)
		for _, s := range extractCodeSmellsFromMeta(n.Metadata) {
			if s != "" && !seen["smell:"+s] {
				seen["smell:"+s] = true
				dc.CodeSmells = append(dc.CodeSmells, s)
			}
		}
	}

	// Code smell nodes linked from anti-patterns via EdgeSmellsLike.
	antiSet := make(map[string]bool, len(antiPatternIDs))
	for _, id := range antiPatternIDs {
		antiSet[id] = true
	}
	for _, e := range g.byKind[EdgeSmellsLike] {
		if !antiSet[e.Src] {
			continue
		}
		n := g.nodes[e.Dst]
		if n == nil || n.Type != NodeTypeCodeSmell {
			continue
		}
		if n.Name != "" && !seen["smell:"+n.Name] {
			seen["smell:"+n.Name] = true
			dc.CodeSmells = append(dc.CodeSmells, n.Name)
		}
	}

	sort.Strings(dc.DesignPatterns)
	sort.Strings(dc.AntiPatterns)
	sort.Strings(dc.CodeSmells)
	return dc, nil
}

// DesignContextForNode returns design patterns and anti-patterns that are
// directly linked to the given node ID.
func (g *Graph) DesignContextForNode(ctx context.Context, nodeID string) (*DesignContext, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	dc := &DesignContext{}
	seen := map[string]bool{}
	var antiPatternIDs []string

	for _, kind := range []string{EdgeImplements, EdgeExhibits, EdgeTouchesFile} {
		for _, e := range g.byKind[kind] {
			if e.Dst != nodeID {
				continue
			}
			n := g.nodes[e.Src]
			if n == nil {
				continue
			}
			switch n.Type {
			case NodeTypeDesignPattern:
				if !seen[n.Name] {
					seen[n.Name] = true
					dc.DesignPatterns = append(dc.DesignPatterns, n.Name)
				}
			case NodeTypeAntiPattern:
				if !seen[n.Name] {
					seen[n.Name] = true
					dc.AntiPatterns = append(dc.AntiPatterns, n.Name)
				}
				antiPatternIDs = append(antiPatternIDs, n.ID)
				for _, s := range extractCodeSmellsFromMeta(n.Metadata) {
					if s != "" && !seen["smell:"+s] {
						seen["smell:"+s] = true
						dc.CodeSmells = append(dc.CodeSmells, s)
					}
				}
			}
		}
	}

	antiSet := make(map[string]bool, len(antiPatternIDs))
	for _, id := range antiPatternIDs {
		antiSet[id] = true
	}
	for _, e := range g.byKind[EdgeSmellsLike] {
		if !antiSet[e.Src] {
			continue
		}
		n := g.nodes[e.Dst]
		if n == nil || n.Type != NodeTypeCodeSmell {
			continue
		}
		if n.Name != "" && !seen["smell:"+n.Name] {
			seen["smell:"+n.Name] = true
			dc.CodeSmells = append(dc.CodeSmells, n.Name)
		}
	}

	sort.Strings(dc.DesignPatterns)
	sort.Strings(dc.AntiPatterns)
	sort.Strings(dc.CodeSmells)
	return dc, nil
}

// extractCodeSmellsFromMeta unpacks the code_smells array from a node's Metadata map.
func extractCodeSmellsFromMeta(meta map[string]any) []string {
	if meta == nil {
		return nil
	}
	raw, ok := meta["code_smells"]
	if !ok {
		return nil
	}
	// Could be []any or []string depending on how it was stored.
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, s := range v {
			if str, ok := s.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return v
	default:
		// Try JSON roundtrip.
		b, _ := json.Marshal(raw)
		var ss []string
		_ = json.Unmarshal(b, &ss)
		return ss
	}
}

// extractCodeSmells parses code_smells from a JSON metadata string (for backward compat).
func extractCodeSmells(metaJSON string) []string {
	var meta map[string]json.RawMessage
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return nil
	}
	rawVal, ok := meta["code_smells"]
	if !ok {
		return nil
	}
	var smells []string
	_ = json.Unmarshal(rawVal, &smells)
	return smells
}

// ---- preflight audit ----

// PreflightAuditRecord is a durable record of one preflight run.
type PreflightAuditRecord struct {
	ID             string   `json:"id"`
	Task           string   `json:"task,omitempty"`
	Timestamp      int64    `json:"timestamp,omitempty"`
	GitSHA         string   `json:"git_sha,omitempty"`
	Files          []string `json:"files,omitempty"`
	ForbiddenFixes []string `json:"forbidden_fixes,omitempty"`
	Invariants     []string `json:"invariants,omitempty"`
	CodeSmells     []string `json:"code_smells,omitempty"`
	CreatedAt      int64    `json:"created_at,omitempty"`
}

// InsertPreflightAudit inserts a durable preflight audit record.
func (g *Graph) InsertPreflightAudit(ctx context.Context, r PreflightAuditRecord) error {
	if g.readOnly {
		return fmt.Errorf("InsertPreflightAudit: graph is read-only")
	}
	if r.ID == "" {
		r.ID = fmt.Sprintf("preflight-audit-%d", time.Now().UnixNano())
	}
	now := time.Now().Unix()
	if r.Timestamp == 0 {
		r.Timestamp = now
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}

	g.preflightMu.Lock()
	// Upsert: replace if same ID.
	replaced := false
	for i, existing := range g.preflights {
		if existing.ID == r.ID {
			cp := r
			g.preflights[i] = &cp
			replaced = true
			break
		}
	}
	if !replaced {
		cp := r
		g.preflights = append(g.preflights, &cp)
	}
	g.preflightMu.Unlock()

	return g.writeJSON("preflight_audits", r.ID, &r)
}

// QueryPreflightAudits returns audit records filtered by since (unix timestamp,
// 0 = no bound) and gitSHA (empty = no filter), ordered by timestamp descending.
func (g *Graph) QueryPreflightAudits(ctx context.Context, since int64, gitSHA string) ([]*PreflightAuditRecord, error) {
	g.preflightMu.RLock()
	var out []*PreflightAuditRecord
	for _, r := range g.preflights {
		if r.Timestamp < since {
			continue
		}
		if gitSHA != "" && r.GitSHA != gitSHA {
			continue
		}
		cp := *r
		out = append(out, &cp)
	}
	g.preflightMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out, nil
}

// AgentUsageEvent is a single recorded preflight/agent-context call.
type AgentUsageEvent struct {
	ID                string `json:"id"`
	EventTime         int64  `json:"event_time,omitempty"`
	Agent             string `json:"agent,omitempty"`
	SessionIDHash     string `json:"session_id_hash,omitempty"`
	Repo              string `json:"repo,omitempty"`
	Tool              string `json:"tool,omitempty"`
	Operation         string `json:"operation,omitempty"`
	ResultStatus      string `json:"result_status,omitempty"`
	Confidence        string `json:"confidence,omitempty"`
	TaskType          string `json:"task_type,omitempty"`
	ChangedFilesCount int    `json:"changed_files_count,omitempty"`
}

// RecordAgentUsage inserts a usage event. Raw prompts are never stored.
func (g *Graph) RecordAgentUsage(ctx context.Context, e AgentUsageEvent) error {
	if g.readOnly {
		return fmt.Errorf("RecordAgentUsage: graph is read-only")
	}
	if e.ID == "" {
		return errors.New("RecordAgentUsage: id required")
	}
	if e.EventTime == 0 {
		e.EventTime = time.Now().Unix()
	}

	g.usageMu.Lock()
	if _, exists := g.usageEvents[e.ID]; !exists {
		cp := e
		g.usageEvents[e.ID] = &cp
	}
	// INSERT OR IGNORE semantics: no-op if ID already exists.
	g.usageMu.Unlock()
	return nil
}

// AgentUsageSummary holds aggregate usage stats over a time window.
type AgentUsageSummary struct {
	WindowDays                   int     `json:"window_days"`
	SessionsTotal                int     `json:"sessions_total"`
	PreflightCalls               int     `json:"preflight_calls"`
	AgentContextCalls            int     `json:"agent_context_calls"`
	ScanViolationsCalls          int     `json:"scan_violations_calls"`
	PreEditContextCalls          int     `json:"pre_edit_context_calls"`
	CommitsWithoutIntegrityCheck int     `json:"commits_without_integrity_check"`
	PreflightSkipRatePct         float64 `json:"preflight_skip_rate_pct"`
	Status                       string  `json:"status"`
	RecommendedAction            string  `json:"recommended_action,omitempty"`
}

// QueryAgentUsageSummary returns aggregate usage stats for a rolling window.
func (g *Graph) QueryAgentUsageSummary(ctx context.Context, windowDays int) (*AgentUsageSummary, error) {
	since := time.Now().AddDate(0, 0, -windowDays).Unix()

	g.usageMu.RLock()
	s := &AgentUsageSummary{WindowDays: windowDays}
	sessions := map[string]bool{}
	for _, e := range g.usageEvents {
		if e.EventTime < since {
			continue
		}
		if e.SessionIDHash != "" {
			sessions[e.SessionIDHash] = true
		}
		if e.Operation == "called" {
			switch e.Tool {
			case "awareness.preflight":
				s.PreflightCalls++
			case "awareness.agent_context":
				s.AgentContextCalls++
			case "awareness.scan_violations":
				s.ScanViolationsCalls++
			case "awareness.pre_edit_context":
				s.PreEditContextCalls++
			}
		}
		if e.Tool == "commit.graph_integrity" && e.Operation == "skipped" {
			s.CommitsWithoutIntegrityCheck++
		}
	}
	g.usageMu.RUnlock()

	s.SessionsTotal = len(sessions)
	if s.SessionsTotal > 0 {
		s.PreflightSkipRatePct = (1 - float64(s.PreflightCalls)/float64(s.SessionsTotal)) * 100
		if s.PreflightSkipRatePct < 0 {
			s.PreflightSkipRatePct = 0
		}
	}

	switch {
	case s.SessionsTotal == 0:
		s.Status = "no_data"
		s.RecommendedAction = "Configure session-start hook to call awareness.agent_context"
	case s.CommitsWithoutIntegrityCheck > 0:
		s.Status = "warning"
		s.RecommendedAction = fmt.Sprintf("%d commits bypassed graph integrity check — run awareness.graph_integrity_check before committing", s.CommitsWithoutIntegrityCheck)
	case s.PreflightSkipRatePct > 50:
		s.Status = "warning"
		s.RecommendedAction = "Configure session-start hook to call awareness.agent_context — skip rate is high"
	default:
		s.Status = "ok"
	}

	return s, nil
}

// placeholders returns n comma-separated "?" — kept for compatibility with
// any callers that might reference this helper.
func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	b := make([]byte, n*2-1)
	for i := range b {
		if i%2 == 0 {
			b[i] = '?'
		} else {
			b[i] = ','
		}
	}
	return string(b)
}

// stringsToInterfaces converts []string to []interface{} — kept for compat.
func stringsToInterfaces(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

package sessionoracle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/google/uuid"
)

// sessionFile is the on-disk format for a session and all its sub-records.
type sessionFile struct {
	ID              string                 `json:"id"`
	Title           string                 `json:"title"`
	Objective       string                 `json:"objective"`
	Actor           string                 `json:"actor"`
	Status          string                 `json:"status"`
	StartedAt       int64                  `json:"started_at"`
	EndedAt         int64                  `json:"ended_at,omitempty"`
	ParentSessionID string                 `json:"parent_session_id,omitempty"`
	RepoRoot        string                 `json:"repo_root"`
	Branch          string                 `json:"branch"`
	GitCommitStart  string                 `json:"git_commit_start"`
	GitCommitEnd    string                 `json:"git_commit_end,omitempty"`
	Decisions       []SessionDecision      `json:"decisions,omitempty"`
	Assumptions     []SessionAssumption    `json:"assumptions,omitempty"`
	FileTouches     []SessionFileTouch     `json:"file_touches,omitempty"`
	Unfinished      []SessionUnfinishedWork `json:"unfinished,omitempty"`
	Warnings        []SessionWarning       `json:"warnings,omitempty"`
	TestResults     []SessionTestResult    `json:"test_results,omitempty"`
	ResumeSnapshots []SessionResumeSnapshot `json:"resume_snapshots,omitempty"`
}

// Oracle is the session resumption store backed by the awareness graph.
type Oracle struct {
	mu      sync.Mutex
	g       *graph.Graph
	dataDir string // <graph.DataDir()>/sessions; "" = in-memory only

	// memSessions holds sessions when dataDir == "".
	memSessions map[string]*sessionFile
}

// New returns an Oracle backed by the given awareness graph.
func New(g *graph.Graph) *Oracle {
	sessDir := ""
	if d := g.DataDir(); d != "" {
		sessDir = filepath.Join(d, "sessions")
	}
	return &Oracle{g: g, dataDir: sessDir, memSessions: make(map[string]*sessionFile)}
}

// sessionsDir ensures the sessions directory exists.
func (o *Oracle) sessionsDir() string {
	if o.dataDir == "" {
		return ""
	}
	_ = os.MkdirAll(o.dataDir, 0o755)
	return o.dataDir
}

// sessionPath returns the file path for a session ID.
func (o *Oracle) sessionPath(id string) string {
	return filepath.Join(o.sessionsDir(), sanitizeSessionID(id)+".json")
}

// sanitizeSessionID converts a session ID to a filesystem-safe filename component.
func sanitizeSessionID(id string) string {
	r := strings.NewReplacer("/", "_", ":", "_", " ", "_")
	return r.Replace(id)
}

// readSession loads a session. Returns error if not found.
func (o *Oracle) readSession(id string) (*sessionFile, error) {
	if o.dataDir == "" {
		// In-memory mode.
		sf, ok := o.memSessions[id]
		if !ok {
			return nil, fmt.Errorf("sessionoracle: in-memory graph, no session %s", id)
		}
		// Return a copy to prevent accidental mutation.
		cp := *sf
		return &cp, nil
	}
	data, err := os.ReadFile(o.sessionPath(id))
	if err != nil {
		return nil, fmt.Errorf("sessionoracle: get session %s: %w", id, err)
	}
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("sessionoracle: decode session %s: %w", id, err)
	}
	return &sf, nil
}

// writeSession writes a session (in-memory or file-backed).
func (o *Oracle) writeSession(sf *sessionFile) error {
	if o.dataDir == "" {
		// In-memory mode: store a copy.
		cp := *sf
		o.memSessions[sf.ID] = &cp
		return nil
	}
	dir := o.sessionsDir()
	if dir == "" {
		return nil
	}
	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}
	path := o.sessionPath(sf.ID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// updateSession loads, mutates, and saves a session atomically.
func (o *Oracle) updateSession(id string, fn func(*sessionFile)) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	sf, err := o.readSession(id)
	if err != nil {
		return err
	}
	fn(sf)
	return o.writeSession(sf)
}

// ── Session lifecycle ─────────────────────────────────────────────────────────

// StartSession creates a new agent session record.
func (o *Oracle) StartSession(ctx context.Context, req StartSessionRequest) (*AgentSession, error) {
	id := req.ID
	if id == "" {
		id = "SES-" + uuid.New().String()[:8]
	}
	actor := req.Actor
	if actor == "" {
		actor = "claude"
	}

	branch, commit := gitBranchAndCommit(req.RepoRoot)

	s := &AgentSession{
		ID:              id,
		Title:           req.Title,
		Objective:       req.Objective,
		Actor:           actor,
		Status:          "open",
		StartedAt:       time.Now().UnixMilli(),
		ParentSessionID: req.ParentSessionID,
		RepoRoot:        req.RepoRoot,
		Branch:          branch,
		GitCommitStart:  commit,
	}

	sf := &sessionFile{
		ID:              s.ID,
		Title:           s.Title,
		Objective:       s.Objective,
		Actor:           s.Actor,
		Status:          s.Status,
		StartedAt:       s.StartedAt,
		ParentSessionID: s.ParentSessionID,
		RepoRoot:        s.RepoRoot,
		Branch:          s.Branch,
		GitCommitStart:  s.GitCommitStart,
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if err := o.writeSession(sf); err != nil {
		return nil, fmt.Errorf("sessionoracle: start session: %w", err)
	}
	return s, nil
}

// GetSession loads a session by ID.
func (o *Oracle) GetSession(ctx context.Context, id string) (*AgentSession, error) {
	sf, err := o.readSession(id)
	if err != nil {
		return nil, err
	}
	return sessionFileToAgentSession(sf), nil
}

func sessionFileToAgentSession(sf *sessionFile) *AgentSession {
	return &AgentSession{
		ID:              sf.ID,
		Title:           sf.Title,
		Objective:       sf.Objective,
		Actor:           sf.Actor,
		Status:          sf.Status,
		StartedAt:       sf.StartedAt,
		EndedAt:         sf.EndedAt,
		ParentSessionID: sf.ParentSessionID,
		RepoRoot:        sf.RepoRoot,
		Branch:          sf.Branch,
		GitCommitStart:  sf.GitCommitStart,
		GitCommitEnd:    sf.GitCommitEnd,
	}
}

// LatestOpenSession returns the most recent open session for the given repo root.
func (o *Oracle) LatestOpenSession(ctx context.Context, repoRoot string) (*AgentSession, error) {
	sessions, err := o.listAllSessions()
	if err != nil {
		return nil, fmt.Errorf("sessionoracle: no session for repo %s: %w", repoRoot, err)
	}

	// Sort by started_at descending.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt > sessions[j].StartedAt
	})

	// First pass: open sessions for this repo.
	for _, sf := range sessions {
		if sf.Status == "open" && sf.RepoRoot == repoRoot {
			return sessionFileToAgentSession(sf), nil
		}
	}
	// Fallback: any session for this repo.
	for _, sf := range sessions {
		if sf.RepoRoot == repoRoot {
			return sessionFileToAgentSession(sf), nil
		}
	}
	return nil, fmt.Errorf("sessionoracle: no session for repo %s", repoRoot)
}

// listAllSessions reads all sessions (from memory or files).
func (o *Oracle) listAllSessions() ([]*sessionFile, error) {
	if o.dataDir == "" {
		// In-memory mode.
		var sessions []*sessionFile
		for _, sf := range o.memSessions {
			cp := *sf
			sessions = append(sessions, &cp)
		}
		return sessions, nil
	}
	dir := o.sessionsDir()
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
	var sessions []*sessionFile
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var sf sessionFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}
		sf2 := sf
		sessions = append(sessions, &sf2)
	}
	return sessions, nil
}

// CloseSession marks a session closed, builds a resume snapshot, and optionally pushes to AI Memory.
func (o *Oracle) CloseSession(ctx context.Context, sessionID string, pushToAIMemory bool, bridge AIMemoryBridge) (*SessionResumeSnapshot, error) {
	_, commit := gitBranchAndCommit("")

	err := o.updateSession(sessionID, func(sf *sessionFile) {
		sf.Status = "closed"
		sf.EndedAt = time.Now().Unix()
		sf.GitCommitEnd = commit
	})
	if err != nil {
		return nil, fmt.Errorf("sessionoracle: close session: %w", err)
	}

	snap, err := o.BuildResumeSnapshot(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if pushToAIMemory && bridge != nil {
		sess, _ := o.GetSession(ctx, sessionID)
		_ = bridge.StoreSessionSummary(ctx, sessionToMemorySummary(sess, snap))
		for _, d := range snap.Decisions {
			if d.Confidence == "high" {
				_ = bridge.StoreDurableDecision(ctx, d)
			}
		}
	}
	return snap, nil
}

// ── Events ────────────────────────────────────────────────────────────────────

// RecordSessionEvent appends a raw event to the session log.
// Events are not persisted to the session file (they are ephemeral).
func (o *Oracle) RecordSessionEvent(ctx context.Context, sessionID, eventType, title, body string, payload interface{}, turnIndex int) error {
	// Events are not persisted in the JSON model (they are high-volume and
	// not needed for session resume). This is a no-op stub.
	return nil
}

// ── File touches ──────────────────────────────────────────────────────────────

// RecordFileTouch persists a file access record and captures fingerprints.
func (o *Oracle) RecordFileTouch(ctx context.Context, sessionID, path, action, reason string, turnIndex int) (*SessionFileTouch, error) {
	fpBefore := fingerprint(path)
	fpAfter := ""
	if action == "edit" || action == "create" || action == "delete" || action == "rename" {
		fpAfter = fpBefore
	}

	id := "FT-" + uuid.New().String()[:8]
	now := time.Now().Unix()

	ft := SessionFileTouch{
		ID:                id,
		SessionID:         sessionID,
		Path:              path,
		Action:            action,
		FingerprintBefore: fpBefore,
		FingerprintAfter:  fpAfter,
		Reason:            reason,
		CreatedAt:         now,
	}

	err := o.updateSession(sessionID, func(sf *sessionFile) {
		ft.Sequence = len(sf.FileTouches) + 1
		sf.FileTouches = append(sf.FileTouches, ft)
	})
	if err != nil {
		return nil, fmt.Errorf("sessionoracle: record file touch: %w", err)
	}

	return &ft, nil
}

// UpdateFileTouchAfter sets the fingerprint_after for a previously recorded touch.
func (o *Oracle) UpdateFileTouchAfter(ctx context.Context, touchID, fingerprintAfter string) error {
	// We need to find and update the touch across all sessions.
	// For efficiency, scan all sessions.
	sessions, err := o.listAllSessions()
	if err != nil {
		return err
	}
	for _, sf := range sessions {
		for i := range sf.FileTouches {
			if sf.FileTouches[i].ID == touchID {
				sessID := sf.ID
				return o.updateSession(sessID, func(sf2 *sessionFile) {
					for j := range sf2.FileTouches {
						if sf2.FileTouches[j].ID == touchID {
							sf2.FileTouches[j].FingerprintAfter = fingerprintAfter
							return
						}
					}
				})
			}
		}
	}
	return nil // touch not found — silently ignore
}

// ListFileTouches returns all file touches for a session in sequence order.
func (o *Oracle) ListFileTouches(ctx context.Context, sessionID string) ([]SessionFileTouch, error) {
	sf, err := o.readSession(sessionID)
	if err != nil {
		return nil, err
	}
	touches := make([]SessionFileTouch, len(sf.FileTouches))
	copy(touches, sf.FileTouches)
	sort.Slice(touches, func(i, j int) bool {
		return touches[i].Sequence < touches[j].Sequence
	})
	return touches, nil
}

// ── Decisions ─────────────────────────────────────────────────────────────────

// RecordDecision persists an architectural decision.
func (o *Oracle) RecordDecision(ctx context.Context, req RecordDecisionRequest) (*SessionDecision, error) {
	if req.Confidence == "" {
		req.Confidence = "medium"
	}
	d := SessionDecision{
		ID:                     "DEC-" + uuid.New().String()[:8],
		SessionID:              req.SessionID,
		Title:                  req.Title,
		Decision:               req.Decision,
		Rationale:              req.Rationale,
		AlternativesConsidered: req.AlternativesConsidered,
		RelatedFiles:           req.RelatedFiles,
		RelatedInvariants:      req.RelatedInvariants,
		RelatedIncidents:       req.RelatedIncidents,
		Confidence:             req.Confidence,
		CreatedAt:              time.Now().Unix(),
	}
	if err := o.updateSession(req.SessionID, func(sf *sessionFile) {
		sf.Decisions = append(sf.Decisions, d)
	}); err != nil {
		return nil, fmt.Errorf("sessionoracle: record decision: %w", err)
	}
	return &d, nil
}

// ListDecisions returns all decisions for a session.
func (o *Oracle) ListDecisions(ctx context.Context, sessionID string) ([]SessionDecision, error) {
	sf, err := o.readSession(sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionDecision, len(sf.Decisions))
	copy(out, sf.Decisions)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out, nil
}

// ── Assumptions ───────────────────────────────────────────────────────────────

// RecordAssumption persists an unverified assumption.
func (o *Oracle) RecordAssumption(ctx context.Context, req RecordAssumptionRequest) (*SessionAssumption, error) {
	a := SessionAssumption{
		ID:             "ASM-" + uuid.New().String()[:8],
		SessionID:      req.SessionID,
		Assumption:     req.Assumption,
		Basis:          req.Basis,
		Status:         "unverified",
		ValidationPlan: req.ValidationPlan,
		RelatedFiles:   req.RelatedFiles,
		CreatedAt:      time.Now().Unix(),
	}
	if err := o.updateSession(req.SessionID, func(sf *sessionFile) {
		sf.Assumptions = append(sf.Assumptions, a)
	}); err != nil {
		return nil, fmt.Errorf("sessionoracle: record assumption: %w", err)
	}
	return &a, nil
}

// ResolveAssumption updates the status of an assumption.
func (o *Oracle) ResolveAssumption(ctx context.Context, assumptionID, status, _ string) error {
	sessions, err := o.listAllSessions()
	if err != nil {
		return err
	}
	for _, sf := range sessions {
		for _, a := range sf.Assumptions {
			if a.ID == assumptionID {
				return o.updateSession(sf.ID, func(sf2 *sessionFile) {
					for i := range sf2.Assumptions {
						if sf2.Assumptions[i].ID == assumptionID {
							sf2.Assumptions[i].Status = status
							sf2.Assumptions[i].ResolvedAt = time.Now().Unix()
							return
						}
					}
				})
			}
		}
	}
	return nil
}

// ListAssumptions returns all assumptions for a session.
func (o *Oracle) ListAssumptions(ctx context.Context, sessionID string) ([]SessionAssumption, error) {
	sf, err := o.readSession(sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionAssumption, len(sf.Assumptions))
	copy(out, sf.Assumptions)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out, nil
}

// ── Unfinished work ───────────────────────────────────────────────────────────

// RecordUnfinishedWork persists an incomplete task.
func (o *Oracle) RecordUnfinishedWork(ctx context.Context, req RecordUnfinishedWorkRequest) (*SessionUnfinishedWork, error) {
	if req.Priority == "" {
		req.Priority = "medium"
	}
	w := SessionUnfinishedWork{
		ID:               "TODO-" + uuid.New().String()[:8],
		SessionID:        req.SessionID,
		Title:            req.Title,
		Description:      req.Description,
		Priority:         req.Priority,
		ReasonUnfinished: req.ReasonUnfinished,
		NextAction:       req.NextAction,
		RelatedFiles:     req.RelatedFiles,
		RelatedTests:     req.RelatedTests,
		RelatedIncidents: req.RelatedIncidents,
		Status:           "open",
		CreatedAt:        time.Now().Unix(),
	}
	if err := o.updateSession(req.SessionID, func(sf *sessionFile) {
		sf.Unfinished = append(sf.Unfinished, w)
	}); err != nil {
		return nil, fmt.Errorf("sessionoracle: record unfinished work: %w", err)
	}
	return &w, nil
}

// CloseUnfinishedWork marks a work item closed.
func (o *Oracle) CloseUnfinishedWork(ctx context.Context, workID, _ string) error {
	sessions, err := o.listAllSessions()
	if err != nil {
		return err
	}
	for _, sf := range sessions {
		for _, w := range sf.Unfinished {
			if w.ID == workID {
				return o.updateSession(sf.ID, func(sf2 *sessionFile) {
					for i := range sf2.Unfinished {
						if sf2.Unfinished[i].ID == workID {
							sf2.Unfinished[i].Status = "closed"
							sf2.Unfinished[i].ClosedAt = time.Now().Unix()
							return
						}
					}
				})
			}
		}
	}
	return nil
}

// priorityOrder returns a sort key for priority strings.
func priorityOrder(p string) int {
	switch p {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

// ListUnfinishedWork returns all open work items for a session.
func (o *Oracle) ListUnfinishedWork(ctx context.Context, sessionID string) ([]SessionUnfinishedWork, error) {
	sf, err := o.readSession(sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionUnfinishedWork, len(sf.Unfinished))
	copy(out, sf.Unfinished)
	sort.Slice(out, func(i, j int) bool {
		pi := priorityOrder(out[i].Priority)
		pj := priorityOrder(out[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out, nil
}

// ── Warnings ──────────────────────────────────────────────────────────────────

// RecordSessionWarning persists a warning that was active during the session.
func (o *Oracle) RecordSessionWarning(ctx context.Context, req RecordSessionWarningRequest) (*SessionWarning, error) {
	w := SessionWarning{
		ID:              "WARN-" + uuid.New().String()[:8],
		SessionID:       req.SessionID,
		WarningType:     req.WarningType,
		Severity:        req.Severity,
		Message:         req.Message,
		RelatedFile:     req.RelatedFile,
		RelatedIncident: req.RelatedIncident,
		CreatedAt:       time.Now().Unix(),
	}
	if err := o.updateSession(req.SessionID, func(sf *sessionFile) {
		sf.Warnings = append(sf.Warnings, w)
	}); err != nil {
		return nil, fmt.Errorf("sessionoracle: record warning: %w", err)
	}
	return &w, nil
}

// ListWarnings returns all warnings for a session.
func (o *Oracle) ListWarnings(ctx context.Context, sessionID string) ([]SessionWarning, error) {
	sf, err := o.readSession(sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionWarning, len(sf.Warnings))
	copy(out, sf.Warnings)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out, nil
}

// ── Test results ──────────────────────────────────────────────────────────────

// RecordTestResult persists the result of a test run.
func (o *Oracle) RecordTestResult(ctx context.Context, req RecordTestResultRequest) (*SessionTestResult, error) {
	r := SessionTestResult{
		ID:            "TEST-" + uuid.New().String()[:8],
		SessionID:     req.SessionID,
		Command:       req.Command,
		Status:        req.Status,
		Summary:       req.Summary,
		OutputExcerpt: req.OutputExcerpt,
		RelatedFiles:  req.RelatedFiles,
		CreatedAt:     time.Now().Unix(),
	}
	if err := o.updateSession(req.SessionID, func(sf *sessionFile) {
		sf.TestResults = append(sf.TestResults, r)
	}); err != nil {
		return nil, fmt.Errorf("sessionoracle: record test result: %w", err)
	}
	return &r, nil
}

// ListTestResults returns all test results for a session.
func (o *Oracle) ListTestResults(ctx context.Context, sessionID string) ([]SessionTestResult, error) {
	sf, err := o.readSession(sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionTestResult, len(sf.TestResults))
	copy(out, sf.TestResults)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// fingerprint returns the sha256 of a file's content, or "" if unreadable.
func fingerprint(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// gitBranchAndCommit returns the current git branch and HEAD commit in the given dir.
func gitBranchAndCommit(repoRoot string) (branch, commit string) {
	args := []string{"rev-parse", "--abbrev-ref", "HEAD"}
	if repoRoot != "" {
		args = append([]string{"-C", repoRoot}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err == nil {
		branch = strings.TrimSpace(string(out))
	}

	args2 := []string{"rev-parse", "HEAD"}
	if repoRoot != "" {
		args2 = append([]string{"-C", repoRoot}, args2...)
	}
	out2, err := exec.Command("git", args2...).Output()
	if err == nil {
		commit = strings.TrimSpace(string(out2))
	}
	return
}

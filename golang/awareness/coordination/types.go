package coordination

// Run status values
const (
	StatusOpen    = "open"
	StatusClosed  = "closed"
	StatusAborted = "aborted"
)

// Agent status values
const (
	AgentActive  = "active"
	AgentIdle    = "idle"
	AgentBlocked = "blocked"
	AgentDone    = "done"
	AgentFailed  = "failed"
	AgentExpired = "expired"
)

// Claim kinds
const (
	ClaimRead        = "read"
	ClaimInvestigate = "investigate"
	ClaimLikelyEdit  = "likely_edit"
	ClaimDoNotTouch  = "do_not_touch"
)

// Lock kinds
const (
	LockEdit             = "edit"
	LockRename           = "rename"
	LockDelete           = "delete"
	LockSemanticBoundary = "semantic_boundary"
	LockDoNotTouch       = "do_not_touch"
)

// Lock/claim status
const (
	StatusActive     = "active"
	StatusReleased   = "released"
	StatusExpired    = "expired"
	StatusSuperseded = "superseded"
)

// TTL defaults (seconds)
const (
	TTLReadClaim        = int64(15 * 60)
	TTLInvestigateClaim = int64(30 * 60)
	TTLLikelyEditClaim  = int64(30 * 60)
	TTLEditLock         = int64(20 * 60)
	TTLDoNotTouchLock   = int64(2 * 60 * 60)
)

// CoordinationRun represents a multi-agent coordination session.
type CoordinationRun struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Objective      string `json:"objective"`
	Status         string `json:"status"`
	OwnerAgentID   string `json:"owner_agent_id,omitempty"`
	RepoRoot       string `json:"repo_root,omitempty"`
	Branch         string `json:"branch,omitempty"`
	GitCommitStart string `json:"git_commit_start,omitempty"`
	GitCommitEnd   string `json:"git_commit_end,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	ClosedAt       int64  `json:"closed_at,omitempty"`
}

// AgentParticipant represents an agent participating in a coordination run.
type AgentParticipant struct {
	ID          string `json:"id"`
	RunID       string `json:"run_id"`
	AgentName   string `json:"agent_name"`
	AgentKind   string `json:"agent_kind"`
	SessionID   string `json:"session_id,omitempty"`
	Role        string `json:"role,omitempty"`
	Status      string `json:"status"`
	HeartbeatAt int64  `json:"heartbeat_at,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// CoordinationWorkItem represents a unit of work in a coordination run.
type CoordinationWorkItem struct {
	ID                string   `json:"id"`
	RunID             string   `json:"run_id"`
	Title             string   `json:"title"`
	Description       string   `json:"description,omitempty"`
	Status            string   `json:"status"`
	Priority          string   `json:"priority"`
	AssignedAgentID   string   `json:"assigned_agent_id,omitempty"`
	ClaimedByAgentID  string   `json:"claimed_by_agent_id,omitempty"`
	RelatedFiles      []string `json:"related_files,omitempty"`
	RelatedComponents []string `json:"related_components,omitempty"`
	RelatedInvariants []string `json:"related_invariants,omitempty"`
	RelatedIncidents  []string `json:"related_incidents,omitempty"`
	CreatedAt         int64    `json:"created_at"`
	UpdatedAt         int64    `json:"updated_at"`
	ClosedAt          int64    `json:"closed_at,omitempty"`
}

// FileClaim represents an agent's intent to read or edit a file.
type FileClaim struct {
	ID         string `json:"id"`
	RunID      string `json:"run_id"`
	AgentID    string `json:"agent_id"`
	Path       string `json:"path"`
	ClaimKind  string `json:"claim_kind"`
	Reason     string `json:"reason,omitempty"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
	ReleasedAt int64  `json:"released_at,omitempty"`
}

// FileLock represents an exclusive edit lock on a file.
type FileLock struct {
	ID                string `json:"id"`
	RunID             string `json:"run_id"`
	AgentID           string `json:"agent_id"`
	Path              string `json:"path"`
	LockKind          string `json:"lock_kind"`
	Reason            string `json:"reason"`
	FingerprintAtLock string `json:"fingerprint_at_lock,omitempty"`
	Status            string `json:"status"`
	CreatedAt         int64  `json:"created_at"`
	ExpiresAt         int64  `json:"expires_at"`
	ReleasedAt        int64  `json:"released_at,omitempty"`
}

// CoordinationDecision represents an architectural or operational decision made during a run.
type CoordinationDecision struct {
	ID                string   `json:"id"`
	RunID             string   `json:"run_id"`
	AgentID           string   `json:"agent_id"`
	Title             string   `json:"title"`
	Decision          string   `json:"decision"`
	Rationale         string   `json:"rationale"`
	Scope             string   `json:"scope"`
	RelatedFiles      []string `json:"related_files,omitempty"`
	RelatedComponents []string `json:"related_components,omitempty"`
	RelatedInvariants []string `json:"related_invariants,omitempty"`
	RelatedIncidents  []string `json:"related_incidents,omitempty"`
	Binding           bool     `json:"binding"`
	SupersededBy      string   `json:"superseded_by,omitempty"`
	CreatedAt         int64    `json:"created_at"`
}

// CoordinationAssumption represents an assumption recorded during a run.
type CoordinationAssumption struct {
	ID             string `json:"id"`
	RunID          string `json:"run_id"`
	AgentID        string `json:"agent_id"`
	Assumption     string `json:"assumption"`
	Basis          string `json:"basis,omitempty"`
	Status         string `json:"status"`
	ValidationPlan string `json:"validation_plan,omitempty"`
	RelatedFiles   string `json:"related_files,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	ResolvedAt     int64  `json:"resolved_at,omitempty"`
}

// CoordinationWarning represents a warning raised during a run.
type CoordinationWarning struct {
	ID               string `json:"id"`
	RunID            string `json:"run_id"`
	AgentID          string `json:"agent_id,omitempty"`
	WarningType      string `json:"warning_type"`
	Severity         string `json:"severity"`
	Message          string `json:"message"`
	RelatedFile      string `json:"related_file,omitempty"`
	RelatedComponent string `json:"related_component,omitempty"`
	RelatedIncident  string `json:"related_incident,omitempty"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"created_at"`
	AcknowledgedAt   int64  `json:"acknowledged_at,omitempty"`
}

// CoordinationHandoffNote represents a handoff from one agent to another.
type CoordinationHandoffNote struct {
	ID           string   `json:"id"`
	RunID        string   `json:"run_id"`
	FromAgentID  string   `json:"from_agent_id"`
	ToAgentID    string   `json:"to_agent_id,omitempty"`
	WorkItemID   string   `json:"work_item_id,omitempty"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	RelatedFiles []string `json:"related_files,omitempty"`
	CreatedAt    int64    `json:"created_at"`
	ReadAt       int64    `json:"read_at,omitempty"`
}

// CoordinationConflict represents a detected or recorded conflict in a run.
type CoordinationConflict struct {
	ID           string `json:"id"`
	RunID        string `json:"run_id"`
	ConflictType string `json:"conflict_type"`
	Severity     string `json:"severity"`
	AgentA       string `json:"agent_a,omitempty"`
	AgentB       string `json:"agent_b,omitempty"`
	Path         string `json:"path,omitempty"`
	Symbol       string `json:"symbol,omitempty"`
	Message      string `json:"message"`
	Resolution   string `json:"resolution,omitempty"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"created_at"`
	ResolvedAt   int64  `json:"resolved_at,omitempty"`
}

// CoordinationSnapshot is the full state of a coordination run at a point in time.
type CoordinationSnapshot struct {
	Run              CoordinationRun           `json:"run"`
	Agents           []AgentParticipant        `json:"agents"`
	WorkItems        []CoordinationWorkItem    `json:"work_items"`
	ActiveClaims     []FileClaim               `json:"active_claims"`
	ActiveLocks      []FileLock                `json:"active_locks"`
	Decisions        []CoordinationDecision    `json:"decisions"`
	Assumptions      []CoordinationAssumption  `json:"assumptions"`
	Warnings         []CoordinationWarning     `json:"warnings"`
	OpenConflicts    []CoordinationConflict    `json:"open_conflicts"`
	HandoffNotes     []CoordinationHandoffNote `json:"handoff_notes"`
	RecommendedRules []string                  `json:"recommended_rules"`
}

// ── Request types ──────────────────────────────────────────────────────────────

// StartCoordinationRunRequest is the input to StartCoordinationRun.
type StartCoordinationRunRequest struct {
	ID           string
	Title        string
	Objective    string
	OwnerAgentID string
	RepoRoot     string
	Branch       string
}

// JoinCoordinationRunRequest is the input to JoinCoordinationRun.
type JoinCoordinationRunRequest struct {
	RunID     string
	AgentName string
	AgentKind string
	SessionID string
	Role      string
}

// CreateWorkItemRequest is the input to CreateWorkItem.
type CreateWorkItemRequest struct {
	RunID             string
	Title             string
	Description       string
	Priority          string
	AssignedAgentID   string
	RelatedFiles      []string
	RelatedComponents []string
	RelatedInvariants []string
	RelatedIncidents  []string
}

// ClaimFileRequest is the input to ClaimFile.
type ClaimFileRequest struct {
	RunID     string
	AgentID   string
	Path      string
	ClaimKind string
	Reason    string
	TTL       int64 // seconds; 0 = use default
}

// AcquireFileLockRequest is the input to AcquireFileLock.
type AcquireFileLockRequest struct {
	RunID    string
	AgentID  string
	Path     string
	LockKind string
	Reason   string
	TTL      int64 // seconds; 0 = use default
}

// RecordDecisionRequest is the input to RecordCoordinationDecision.
type RecordDecisionRequest struct {
	RunID             string
	AgentID           string
	Title             string
	Decision          string
	Rationale         string
	Scope             string
	RelatedFiles      []string
	RelatedComponents []string
	RelatedInvariants []string
	RelatedIncidents  []string
	Binding           bool
}

// RecordWarningRequest is the input to RecordCoordinationWarning.
type RecordWarningRequest struct {
	RunID            string
	AgentID          string
	WarningType      string
	Severity         string
	Message          string
	RelatedFile      string
	RelatedComponent string
	RelatedIncident  string
}

// RecordHandoffRequest is the input to RecordHandoff.
type RecordHandoffRequest struct {
	RunID        string
	FromAgentID  string
	ToAgentID    string
	WorkItemID   string
	Title        string
	Body         string
	RelatedFiles []string
}

// LockConflict is returned when a lock acquisition fails.
type LockConflict struct {
	Type         string `json:"type"`
	Path         string `json:"path"`
	OwnerAgentID string `json:"owner_agent_id"`
	Message      string `json:"message"`
}

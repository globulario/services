package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	globular "github.com/globulario/services/golang/globular_service"
)

// Notifier sends notifications about incident lifecycle events.
type Notifier interface {
	Notify(ctx context.Context, n *Notification) error
}

// NotificationType indicates what happened.
type NotificationType string

const (
	NotifyApprovalRequired NotificationType = "approval_required"
	NotifyResolved         NotificationType = "resolved"
	NotifyFailed           NotificationType = "failed"
	NotifyExpired          NotificationType = "expired"
	NotifyDenied           NotificationType = "denied"
)

// Notification contains all context an operator needs to act.
type Notification struct {
	Type           NotificationType
	IncidentID     string
	Service        string
	Severity       string
	Summary        string
	RootCause      string
	Confidence     float32
	ProposedAction string
	Rationale      string
	ExpiresAt      time.Time
	ApproveCmd     string
	DenyCmd        string
}

// multiNotifier fans out to multiple notifier implementations.
type multiNotifier struct {
	notifiers []Notifier
}

func newMultiNotifier() *multiNotifier {
	return &multiNotifier{
		notifiers: []Notifier{
			&logNotifier{},
			&eventNotifier{},
		},
	}
}

func (mn *multiNotifier) notify(ctx context.Context, n *Notification) {
	for _, notifier := range mn.notifiers {
		if err := notifier.Notify(ctx, n); err != nil {
			logger.Warn("notification failed", "type", n.Type, "notifier", fmt.Sprintf("%T", notifier), "err", err)
		}
	}
}

// buildNotification creates a Notification from a job.
func buildNotification(job *ai_executorpb.Job, ntype NotificationType) *Notification {
	n := &Notification{
		Type:       ntype,
		IncidentID: job.GetIncidentId(),
		Severity:   "warning",
	}

	if d := job.GetDiagnosis(); d != nil {
		n.Summary = d.GetSummary()
		n.RootCause = d.GetRootCause()
		n.Confidence = d.GetConfidence()
		n.ProposedAction = d.GetProposedAction()
		n.Rationale = d.GetActionReason()
	}

	if job.ExpiresAtMs > 0 {
		n.ExpiresAt = time.UnixMilli(job.ExpiresAtMs)
	}

	n.ApproveCmd = fmt.Sprintf("globular ai approve %s", job.GetIncidentId())
	n.DenyCmd = fmt.Sprintf("globular ai deny %s --reason '<reason>'", job.GetIncidentId())

	return n
}

// --- LogNotifier: structured slog output (always enabled) ---

type logNotifier struct{}

func (ln *logNotifier) Notify(_ context.Context, n *Notification) error {
	switch n.Type {
	case NotifyApprovalRequired:
		logger.Warn("APPROVAL REQUIRED",
			"incident_id", n.IncidentID,
			"summary", n.Summary,
			"root_cause", n.RootCause,
			"confidence", n.Confidence,
			"proposed_action", n.ProposedAction,
			"rationale", n.Rationale,
			"expires", n.ExpiresAt.Format(time.RFC3339),
			"approve_cmd", n.ApproveCmd,
			"deny_cmd", n.DenyCmd,
		)
	case NotifyResolved:
		logger.Info("INCIDENT RESOLVED",
			"incident_id", n.IncidentID,
			"summary", n.Summary,
			"root_cause", n.RootCause,
		)
	case NotifyFailed:
		logger.Error("REMEDIATION FAILED",
			"incident_id", n.IncidentID,
			"summary", n.Summary,
			"proposed_action", n.ProposedAction,
		)
	case NotifyExpired:
		logger.Warn("APPROVAL EXPIRED",
			"incident_id", n.IncidentID,
			"summary", n.Summary,
		)
	case NotifyDenied:
		logger.Info("ACTION DENIED",
			"incident_id", n.IncidentID,
			"summary", n.Summary,
		)
	}
	return nil
}

// --- EventNotifier: publishes to event bus for admin UI / subscribers ---

type eventNotifier struct{}

func (en *eventNotifier) Notify(_ context.Context, n *Notification) error {
	eventName := "alert.incident." + string(n.Type)

	// Build a human-readable message for the admin dashboard.
	message := en.buildHumanMessage(n)

	payload := map[string]interface{}{
		"severity":        n.Severity,
		"incident_id":     n.IncidentID,
		"message":         message,
		"summary":         n.Summary,
		"root_cause":      n.RootCause,
		"confidence":      n.Confidence,
		"proposed_action": n.ProposedAction,
		"rationale":       n.Rationale,
		"service":         "ai_executor",
	}

	if n.Type == NotifyApprovalRequired {
		payload["approve_cmd"] = n.ApproveCmd
		payload["deny_cmd"] = n.DenyCmd
		payload["expires_at"] = n.ExpiresAt.Format(time.RFC3339)
		payload["severity"] = "ERROR"
	}

	// Resolved incidents are informational, not urgent.
	if n.Type == NotifyResolved {
		payload["severity"] = "INFO"
	}

	globular.PublishEvent(eventName, payload)

	return nil
}

// buildHumanMessage creates a readable message for the admin dashboard.
func (en *eventNotifier) buildHumanMessage(n *Notification) string {
	rootCause := n.RootCause
	action := humanActionName(n.ProposedAction)
	confidence := fmt.Sprintf("%d%%", int(n.Confidence*100))

	switch n.Type {
	case NotifyResolved:
		if rootCause != "" {
			return fmt.Sprintf("Diagnosed %s (%s confidence). Action: %s. Incident resolved.",
				rootCause, confidence, action)
		}
		return fmt.Sprintf("Incident resolved. Action: %s.", action)

	case NotifyApprovalRequired:
		return fmt.Sprintf("Detected %s (%s confidence). Proposed: %s — awaiting admin approval. Run: %s",
			rootCause, confidence, action, n.ApproveCmd)

	case NotifyFailed:
		return fmt.Sprintf("Failed to remediate %s. Action %s failed. Manual intervention required.",
			rootCause, action)

	case NotifyExpired:
		return fmt.Sprintf("Approval expired for %s. Proposed action %s was not approved in time.",
			rootCause, action)

	case NotifyDenied:
		return fmt.Sprintf("Admin denied %s for %s.", action, rootCause)

	default:
		return n.Summary
	}
}

func humanActionName(action string) string {
	base := action
	target := ""
	if i := strings.Index(action, ":"); i >= 0 {
		base = action[:i]
		target = action[i+1:]
	}
	names := map[string]string{
		"restart_service": "restart service",
		"notify_admin":    "notify admin",
		"observe_and_record": "observe and record",
		"drain_endpoint":  "drain endpoint",
		"block_ip":        "block IP",
	}
	name := names[base]
	if name == "" {
		name = action
	}
	if target != "" {
		name = fmt.Sprintf("%s %q", name, target)
	}
	return name
}

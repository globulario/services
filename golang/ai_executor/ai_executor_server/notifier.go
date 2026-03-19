package main

import (
	"context"
	"encoding/json"
	"fmt"
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

	payload := map[string]interface{}{
		"severity":        n.Severity,
		"incident_id":     n.IncidentID,
		"summary":         n.Summary,
		"root_cause":      n.RootCause,
		"confidence":      n.Confidence,
		"proposed_action":  n.ProposedAction,
		"rationale":       n.Rationale,
	}

	if n.Type == NotifyApprovalRequired {
		payload["approve_cmd"] = n.ApproveCmd
		payload["deny_cmd"] = n.DenyCmd
		payload["expires_at"] = n.ExpiresAt.Format(time.RFC3339)
		payload["severity"] = "ERROR" // approvals are urgent
	}

	data, _ := json.Marshal(payload)
	globular.PublishEvent(eventName, nil)
	// Re-publish with actual data.
	globular.PublishEvent(eventName, payload)
	_ = data // suppress unused if PublishEvent handles map directly

	return nil
}

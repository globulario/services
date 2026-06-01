package main

// audit.go — publish structured audit events to the Event service for trust-sensitive operations.
//
// Events are published best-effort — audit failure must never block the primary operation.

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/peer"
)

const auditChannelPrefix = "pkg."

// publishAuditEvent sends a structured audit event to the Event service.
// Best-effort: errors are logged but never returned to callers.
func (srv *server) publishAuditEvent(ctx context.Context, eventName string, fields map[string]any) {
	// Enrich with common fields.
	if fields == nil {
		fields = make(map[string]any)
	}
	now := time.Now().UTC()
	fields["event"] = eventName
	fields["timestamp"] = now.Format(time.RFC3339)
	fields["timestamp_unix"] = now.Unix()
	fields["correlation_id"] = Utility.RandomUUID()

	if authCtx := security.FromContext(ctx); authCtx != nil {
		fields["subject"] = authCtx.Subject
		fields["principal_type"] = authCtx.PrincipalType
		fields["auth_method"] = authCtx.AuthMethod
	}

	// Extract source IP from gRPC peer info.
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		fields["source_ip"] = p.Addr.String()
	}

	data, err := json.Marshal(fields)
	if err != nil {
		slog.Debug("audit event marshal failed", "event", eventName, "err", err)
		return
	}

	client, err := srv.getEventClient()
	if err != nil {
		slog.Debug("audit event publish skipped (no event client)", "event", eventName, "err", err)
		return
	}

	channel := auditChannelPrefix + eventName
	if pubErr := client.Publish(channel, data); pubErr != nil {
		slog.Debug("audit event publish failed", "event", eventName, "err", pubErr)
	}
}

// getEventClient returns a connected Event service client.
func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

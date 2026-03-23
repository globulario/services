package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular_client "github.com/globulario/services/golang/globular_client"
	Utility "github.com/globulario/utility"
)

// eventHandler subscribes to operation events and executes actions
// that require root privileges (restart, stop, etc.).
type eventHandler struct {
	srv *NodeAgentServer
}

func newEventHandler(srv *NodeAgentServer) *eventHandler {
	return &eventHandler{srv: srv}
}

// run connects to the event service and subscribes to operation events.
func (eh *eventHandler) run(ctx context.Context) {
	// Wait for services to stabilize.
	time.Sleep(15 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
		c, err := globular_client.GetClient(discoverServiceAddr(10010), "event.EventService", "NewEventService_Client")
		if err != nil {
			log.Printf("event-handler: event service unavailable, retrying in 10s: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}
		client := c.(*event_client.Event_Client)

		log.Printf("event-handler: connected to event service")

		subscriberID := "node_agent_handler_" + eh.srv.nodeID
		if err := client.Subscribe("operation.*", subscriberID, func(evt *eventpb.Event) {
			eh.handleEvent(ctx, evt)
		}); err != nil {
			log.Printf("event-handler: subscribe failed: %v, retrying in 10s", err)
			time.Sleep(10 * time.Second)
			continue
		}

		log.Printf("event-handler: subscribed to operation.*")

		// Block until context cancelled.
		<-ctx.Done()
		client.Close()
		return
	}
}

// handleEvent dispatches operation events to the appropriate handler.
func (eh *eventHandler) handleEvent(ctx context.Context, evt *eventpb.Event) {
	switch evt.GetName() {
	case "operation.restart_requested":
		eh.handleRestart(ctx, evt)
	}
}

// handleRestart restarts a globular service unit.
// The target field from the executor is "restart_service:<unit>" or just "<unit>".
func (eh *eventHandler) handleRestart(ctx context.Context, evt *eventpb.Event) {
	var payload struct {
		Target string `json:"target"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(evt.GetData(), &payload); err != nil {
		log.Printf("event-handler: restart: bad payload: %v", err)
		return
	}

	if payload.Target == "" {
		log.Printf("event-handler: restart: missing target")
		return
	}

	// Extract unit name from "restart_service:<unit>" format.
	unit := payload.Target
	if strings.Contains(unit, ":") {
		unit = unit[strings.LastIndex(unit, ":")+1:]
	}

	// Ensure it's a proper systemd unit name.
	if !strings.HasPrefix(unit, "globular-") {
		unit = "globular-" + strings.ReplaceAll(unit, "_", "-") + ".service"
	}
	if !strings.HasSuffix(unit, ".service") {
		unit = unit + ".service"
	}

	log.Printf("event-handler: restarting %s (requested by %s)", unit, payload.Source)

	if err := eh.srv.performRestartUnits([]string{unit}, nil); err != nil {
		log.Printf("event-handler: restart failed: %s: %v", unit, err)
		return
	}

	log.Printf("event-handler: restart succeeded: %s", unit)
}

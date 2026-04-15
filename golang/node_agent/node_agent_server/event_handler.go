package main

import (
	"context"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
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
		eventAddr := config.ResolveLocalServiceAddr("event.EventService")
		if eventAddr == "" {
			log.Printf("event-handler: event service not found in registry, retrying in 10s")
			time.Sleep(10 * time.Second)
			continue
		}
		c, err := globular_client.GetClient(eventAddr, "event.EventService", "NewEventService_Client")
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
// NOTE: operation.restart_requested is handled by the cluster controller,
// which validates the request, tracks it as a workflow, and calls
// ControlService RPC on the target node agent. The node agent MUST NOT
// act on restart_requested directly — that would bypass the controller's
// policy enforcement, cooldown checks, and audit trail.
func (eh *eventHandler) handleEvent(ctx context.Context, evt *eventpb.Event) {
	switch evt.GetName() {
	case "operation.restart_requested":
		log.Printf("event-handler: ignoring %s (handled by cluster controller)", evt.GetName())
	}
}

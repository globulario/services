package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_router/ai_routerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular_client "github.com/globulario/services/golang/globular_client"
	Utility "github.com/globulario/utility"
)

// anomalyTracker subscribes to ai_watcher security events and maintains
// per-service anomaly scores that feed into the routing scorer.
type anomalyTracker struct {
	mu sync.RWMutex

	// Active anomaly signals: service name → anomaly entry.
	signals map[string]*anomalySignal

	// Per-IP threat tracking (from DoS/Slowloris events).
	threatIPs map[string]*threatEntry

	eventClient *event_client.Event_Client
}

type anomalySignal struct {
	Score     float64   // 0.0-1.0
	Reason    string
	UpdatedAt time.Time
}

type threatEntry struct {
	IP        string
	Reason    string    // dos_rate_exceeded, slowloris_detected
	Service   string    // which service reported it
	DetectedAt time.Time
}

const (
	anomalyDecayDuration = 5 * time.Minute // signals decay after 5 minutes
	anomalyBoostDos      = 0.8
	anomalyBoostSlowloris = 0.6
	anomalyBoostErrorSpike = 0.7
	anomalyBoostAuthDenied = 0.3
	anomalyBoostAuthFailed = 0.4
)

func newAnomalyTracker() *anomalyTracker {
	return &anomalyTracker{
		signals:   make(map[string]*anomalySignal),
		threatIPs: make(map[string]*threatEntry),
	}
}

// start connects to the event service and subscribes to security alerts.
func (at *anomalyTracker) start() {
	// Give services time to start.
	time.Sleep(20 * time.Second)

	addr := config.ResolveServiceAddr("event.EventService", "localhost:10010")
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(addr, "event.EventService", "NewEventService_Client")
	if err != nil {
		logger.Warn("anomaly_tracker: event service unavailable", "err", err)
		return
	}
	client, ok := c.(*event_client.Event_Client)
	if !ok {
		return
	}
	at.eventClient = client

	// Subscribe to security alerts.
	topics := []string{"alert.*"}
	for _, topic := range topics {
		id := "ai_router_anomaly_" + topic
		if err := client.Subscribe(topic, id, at.handleEvent); err != nil {
			logger.Warn("anomaly_tracker: subscribe failed", "topic", topic, "err", err)
		} else {
			logger.Info("anomaly_tracker: subscribed", "topic", topic)
		}
	}

	// Decay loop: reduce stale signals every 30 seconds.
	go at.decayLoop()
}

// handleEvent processes incoming security alert events.
func (at *anomalyTracker) handleEvent(evt *eventpb.Event) {
	if evt == nil {
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(evt.Data, &payload); err != nil {
		return
	}

	service, _ := payload["service"].(string)
	remoteAddr, _ := payload["remote_addr"].(string)
	reason, _ := payload["reason"].(string)

	at.mu.Lock()
	defer at.mu.Unlock()

	now := time.Now()

	switch evt.Name {
	case "alert.dos.detected":
		if service != "" {
			at.signals[service] = &anomalySignal{
				Score: anomalyBoostDos, Reason: "dos_detected", UpdatedAt: now,
			}
		}
		if remoteAddr != "" {
			at.threatIPs[remoteAddr] = &threatEntry{
				IP: remoteAddr, Reason: "dos", Service: service, DetectedAt: now,
			}
		}

	case "alert.slowloris.detected":
		if service != "" {
			at.signals[service] = &anomalySignal{
				Score: anomalyBoostSlowloris, Reason: "slowloris_detected", UpdatedAt: now,
			}
		}
		if remoteAddr != "" {
			at.threatIPs[remoteAddr] = &threatEntry{
				IP: remoteAddr, Reason: "slowloris", Service: service, DetectedAt: now,
			}
		}

	case "alert.error.spike":
		if service != "" {
			at.signals[service] = &anomalySignal{
				Score: anomalyBoostErrorSpike, Reason: "error_spike", UpdatedAt: now,
			}
		}

	case "alert.auth.denied":
		if service != "" {
			// Lower boost — auth denials are common during bootstrap.
			existing := at.signals[service]
			if existing == nil || existing.Score < anomalyBoostAuthDenied {
				at.signals[service] = &anomalySignal{
					Score: anomalyBoostAuthDenied, Reason: reason, UpdatedAt: now,
				}
			}
		}

	case "alert.auth.failed":
		// Failed logins — moderate boost, could be brute force.
		at.signals["authentication.AuthenticationService"] = &anomalySignal{
			Score: anomalyBoostAuthFailed, Reason: "auth_failed", UpdatedAt: now,
		}
	}
}

// getAnomalyScore returns the current anomaly score for a service (0.0-1.0).
func (at *anomalyTracker) getAnomalyScore(service string) float64 {
	at.mu.RLock()
	defer at.mu.RUnlock()

	sig := at.signals[service]
	if sig == nil {
		return 0
	}

	// Decay: reduce score linearly over anomalyDecayDuration.
	age := time.Since(sig.UpdatedAt)
	if age >= anomalyDecayDuration {
		return 0
	}
	decay := 1.0 - (float64(age) / float64(anomalyDecayDuration))
	return sig.Score * decay
}

// getThreatIPs returns currently tracked threat source IPs.
func (at *anomalyTracker) getThreatIPs() []threatEntry {
	at.mu.RLock()
	defer at.mu.RUnlock()

	var entries []threatEntry
	for _, t := range at.threatIPs {
		if time.Since(t.DetectedAt) < anomalyDecayDuration {
			entries = append(entries, *t)
		}
	}
	return entries
}

// decayLoop cleans up expired signals every 30 seconds.
func (at *anomalyTracker) decayLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		at.mu.Lock()
		now := time.Now()
		for k, sig := range at.signals {
			if now.Sub(sig.UpdatedAt) >= anomalyDecayDuration {
				delete(at.signals, k)
			}
		}
		for k, t := range at.threatIPs {
			if now.Sub(t.DetectedAt) >= anomalyDecayDuration {
				delete(at.threatIPs, k)
			}
		}
		at.mu.Unlock()
	}
}

// publishRoutingEvent publishes routing decision events via the event client.
func (at *anomalyTracker) publishRoutingEvent(name string, payload map[string]interface{}) {
	if at.eventClient == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = at.eventClient.Publish(name, data)
}

// serviceClassForScoring returns the service class, checking anomaly context.
// During active DoS, stream-heavy services should be treated more conservatively.
func (at *anomalyTracker) adjustClassForAnomaly(
	class ai_routerpb.ServiceClass,
	service string,
) ai_routerpb.ServiceClass {
	at.mu.RLock()
	defer at.mu.RUnlock()

	sig := at.signals[service]
	if sig == nil {
		return class
	}

	// During active DoS on a stateless service, treat it as deployment-sensitive
	// (slower drain, more careful weight changes).
	if sig.Reason == "dos_detected" && class == ai_routerpb.ServiceClass_STATELESS_UNARY {
		return ai_routerpb.ServiceClass_DEPLOYMENT_SENSITIVE
	}

	return class
}

package main

import (
	"github.com/globulario/services/golang/ai_router/ai_routerpb"
)

// applyStabilityControls adds outlier detection and circuit breaker overrides
// to a service policy when conditions warrant it.
// Phase 2: conservative defaults that protect against cascade failures.
func applyStabilityControls(sp *ai_routerpb.ServicePolicy, class ai_routerpb.ServiceClass, avgErrorRate float64) {
	if sp == nil {
		return
	}

	// Outlier detection: enable for all services.
	// Envoy will auto-eject endpoints returning consecutive 5xx.
	sp.OutlierDetection = &ai_routerpb.OutlierDetectionOverride{
		Consecutive_5Xx:     5,      // eject after 5 consecutive errors
		IntervalMs:          10000,  // check every 10s
		BaseEjectionTimeMs:  30000,  // eject for 30s
		MaxEjectionPercent:  50,     // never eject more than half
	}

	// Control plane: more conservative — eject less aggressively.
	if class == ai_routerpb.ServiceClass_CONTROL_PLANE {
		sp.OutlierDetection.Consecutive_5Xx = 10
		sp.OutlierDetection.MaxEjectionPercent = 30
	}

	// Circuit breakers: tighten under high error rates.
	if avgErrorRate > 0.1 { // >10% error rate
		sp.CircuitBreaker = &ai_routerpb.CircuitBreakerOverride{
			MaxConnections:     512,
			MaxPendingRequests: 256,
			MaxRequests:        512,
			MaxRetries:         1, // reduce retries to stop amplification
		}
		sp.Reasons = append(sp.Reasons, "circuit breakers tightened: error rate elevated")
	}

	if avgErrorRate > 0.3 { // >30% error rate — potential cascade
		sp.CircuitBreaker = &ai_routerpb.CircuitBreakerOverride{
			MaxConnections:     256,
			MaxPendingRequests: 64,
			MaxRequests:        256,
			MaxRetries:         0, // no retries during cascade
		}
		sp.Reasons = append(sp.Reasons, "circuit breakers critical: possible cascade failure")
	}

	// Retry policy: only for unary services, disabled during high error rate.
	if class == ai_routerpb.ServiceClass_STATELESS_UNARY && avgErrorRate < 0.1 {
		sp.RetryPolicy = &ai_routerpb.RetryPolicyOverride{
			RetryOn:    "5xx,reset,connect-failure,refused-stream",
			NumRetries: 2,
			BackoffMs:  100,
		}
	}
}

// computeAvgErrorRate returns the average error rate across all endpoints of a service.
func computeAvgErrorRate(endpoints map[string]*endpointMetrics, serviceName string) float64 {
	var total float64
	var count int
	for _, ep := range endpoints {
		if ep.Service == serviceName {
			total += ep.ErrorRate
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

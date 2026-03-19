package main

import ai_watcherpb "github.com/globulario/services/golang/ai_watcher/ai_watcherpb"

// defaultConfig returns the initial watcher configuration with sensible rules.
func defaultWatcherConfig() *ai_watcherpb.WatcherConfig {
	return &ai_watcherpb.WatcherConfig{
		Enabled: true,
		Paused:  false,

		// Subscribe to these event topics from the event service.
		SubscribeTopics: []string{
			"cluster.*",   // Doctor findings, health changes
			"service.*",   // Service start/stop/crash events
			"node.*",      // Node agent status changes
			"alert.*",     // Monitoring alerts
			"operation.*", // Plan execution phase changes
		},

		// Event rules — what triggers investigation.
		Rules: []*ai_watcherpb.EventRule{
			{
				Id:                  "service-crash",
				EventPattern:        "service.exited",
				Description:         "Service exited with non-zero code",
				Enabled:             true,
				Tier:                ai_watcherpb.PermissionTier_OBSERVE,
				CooldownSeconds:     60,
				BatchWindowSeconds:  10,
				SeverityMin:         "error",
				RepeatThreshold:     1,
			},
			{
				Id:                  "health-check-fail",
				EventPattern:        "cluster.health.degraded",
				Description:         "Cluster health check reports degraded state",
				Enabled:             true,
				Tier:                ai_watcherpb.PermissionTier_OBSERVE,
				CooldownSeconds:     120,
				BatchWindowSeconds:  15,
				SeverityMin:         "warning",
				RepeatThreshold:     1,
			},
			{
				Id:                  "drift-detected",
				EventPattern:        "cluster.drift.*",
				Description:         "Desired state differs from installed state",
				Enabled:             true,
				Tier:                ai_watcherpb.PermissionTier_OBSERVE,
				CooldownSeconds:     300,
				BatchWindowSeconds:  30,
				SeverityMin:         "warning",
				RepeatThreshold:     1,
			},
			{
				Id:                  "convergence-stalled",
				EventPattern:        "operation.stalled",
				Description:         "Plan execution stalled for extended period",
				Enabled:             true,
				Tier:                ai_watcherpb.PermissionTier_OBSERVE,
				CooldownSeconds:     300,
				BatchWindowSeconds:  30,
				SeverityMin:         "error",
				RepeatThreshold:     1,
			},
			{
				Id:                  "cert-expiry-warning",
				EventPattern:        "node.cert.expiring",
				Description:         "TLS certificate approaching expiry",
				Enabled:             true,
				Tier:                ai_watcherpb.PermissionTier_OBSERVE,
				CooldownSeconds:     3600,
				BatchWindowSeconds:  60,
				SeverityMin:         "warning",
				RepeatThreshold:     1,
			},
			{
				Id:                  "doctor-finding",
				EventPattern:        "cluster.finding.*",
				Description:         "Cluster doctor reported a new finding",
				Enabled:             true,
				Tier:                ai_watcherpb.PermissionTier_OBSERVE,
				CooldownSeconds:     120,
				BatchWindowSeconds:  15,
				SeverityMin:         "warning",
				RepeatThreshold:     1,
			},
		},

		// Auto-remediation rules — Tier 2 (disabled by default, user opts in).
		AutoRemediation: []*ai_watcherpb.AutoRemediationRule{
			{
				Id:             "restart-crashed",
				Action:         "restart_service",
				Description:    "Restart a service that crashed and isn't self-healing",
				Enabled:        false, // User must explicitly enable
				MaxRetries:     3,
				TargetServices: []string{"*"},
			},
			{
				Id:             "clear-corrupted-wal",
				Action:         "clear_corrupted_storage",
				Description:    "Delete corrupted WAL/manifest files (BadgerDB, Prometheus TSDB)",
				Enabled:        false,
				MaxRetries:     1,
				TargetServices: []string{"*"},
			},
			{
				Id:             "renew-cert",
				Action:         "cert_renew",
				Description:    "Renew TLS certificates before expiry",
				Enabled:        false,
				MaxRetries:     1,
				TargetServices: []string{"*"},
			},
		},

		Notifications: &ai_watcherpb.NotificationConfig{
			Channels: []string{"log", "event"},
		},

		GlobalCooldownSeconds: 30,
	}
}

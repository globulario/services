package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricsJobsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "globular_backup_jobs_total",
		Help: "Total backup jobs by final state (queued, succeeded, failed, canceled, restore_queued, restore_succeeded, restore_failed, restore_canceled, retention)",
	}, []string{"state"})

	metricsLastSuccess = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "globular_backup_last_success_timestamp",
		Help: "Unix timestamp of last successful backup",
	})

	metricsLastDuration = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "globular_backup_last_duration_seconds",
		Help: "Duration of last completed backup in seconds",
	})

	metricsArtifactsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "globular_backup_artifacts_total",
		Help: "Current number of stored backup artifacts",
	})

	metricsRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "globular_backup_running",
		Help: "Number of backup/restore jobs currently running",
	})

	metricsRetentionDeleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "globular_backup_retention_deleted_total",
		Help: "Total backups deleted by retention policy",
	})

	metricsValidationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "globular_backup_validations_total",
		Help: "Total backup validations by result",
	}, []string{"result"}) // "valid", "invalid"

	metricsRestoreTestedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "globular_backup_restore_tested_total",
		Help: "Total backups that passed restore tests",
	})

	metricsPromotedTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "globular_backup_promoted_total",
		Help: "Current number of promoted (protected) backups",
	})
)

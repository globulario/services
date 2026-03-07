package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// startScheduler launches a background goroutine that triggers cluster backups
// at the configured interval. It returns a cancel function to stop the loop.
// If ScheduleInterval is empty or "0", no scheduler is started.
func (srv *server) startScheduler() context.CancelFunc {
	interval := srv.parseScheduleInterval()
	if interval <= 0 {
		slog.Info("backup scheduler disabled (ScheduleInterval not set)")
		return func() {}
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		slog.Info("backup scheduler started", "interval", interval.String())

		// Wait one interval before the first scheduled backup.
		timer := time.NewTimer(interval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("backup scheduler stopped")
				return
			case <-timer.C:
				srv.runScheduledBackup()
				timer.Reset(interval)
			}
		}
	}()

	return cancel
}

func (srv *server) runScheduledBackup() {
	// Skip if a job is already running.
	if srv.active.count() >= srv.MaxConcurrentJobs {
		slog.Info("scheduled backup skipped: a job is already running")
		return
	}

	slog.Info("scheduled backup starting")

	_, err := srv.RunBackup(context.Background(), &backup_managerpb.RunBackupRequest{
		Plan: &backup_managerpb.BackupPlan{
			Name: "scheduled-backup",
		},
		Mode:          backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER,
		FailIfRunning: true,
	})
	if err != nil {
		slog.Warn("scheduled backup failed to start", "error", err)
		return
	}
	slog.Info("scheduled backup triggered")
}

// parseScheduleInterval parses the ScheduleInterval config string into a
// time.Duration. Supported formats: Go duration ("6h", "24h", "30m") or
// shorthand like "daily" / "weekly". Returns 0 if disabled.
func (srv *server) parseScheduleInterval() time.Duration {
	s := srv.ScheduleInterval
	if s == "" || s == "0" || s == "off" || s == "disabled" {
		return 0
	}

	switch s {
	case "daily":
		return 24 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	case "hourly":
		return time.Hour
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn("invalid ScheduleInterval, scheduler disabled", "value", s, "error", err)
		return 0
	}
	if d < 15*time.Minute {
		slog.Warn("ScheduleInterval too short, clamping to 15m", "value", s)
		d = 15 * time.Minute
	}
	return d
}

package main

import (
	"context"
	"fmt"
	"time"

	backup_managerpb "github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// ── Enum normalization maps ─────────────────────────────────────────────────

var backupJobStateNames = map[int32]string{
	0: "unknown",
	1: "queued",
	2: "running",
	3: "succeeded",
	4: "failed",
	5: "canceled",
}

var backupJobTypeNames = map[int32]string{
	0: "unknown",
	1: "backup",
	2: "restore",
	3: "retention",
}

var backupModeNames = map[int32]string{
	0: "unknown",
	1: "service",
	2: "cluster",
}

var qualityStateNames = map[int32]string{
	0: "unknown",
	1: "unverified",
	2: "validated",
	3: "restore_tested",
	4: "promoted",
}

var backupProviderTypeNames = map[int32]string{
	0: "unknown",
	1: "etcd",
	2: "restic",
	3: "minio",
	4: "scylla",
}

var backupSeverityNames = map[int32]string{
	0: "unknown",
	1: "info",
	2: "warn",
	3: "error",
}

func normalizeJobState(s backup_managerpb.BackupJobState) string {
	if n, ok := backupJobStateNames[int32(s)]; ok {
		return n
	}
	return "unknown"
}

func normalizeJobType(t backup_managerpb.BackupJobType) string {
	if n, ok := backupJobTypeNames[int32(t)]; ok {
		return n
	}
	return "unknown"
}

func normalizeBackupMode(m backup_managerpb.BackupMode) string {
	if n, ok := backupModeNames[int32(m)]; ok {
		return n
	}
	return "unknown"
}

func normalizeQualityState(q backup_managerpb.QualityState) string {
	if n, ok := qualityStateNames[int32(q)]; ok {
		return n
	}
	return "unknown"
}

func normalizeProviderType(p backup_managerpb.BackupProviderType) string {
	if n, ok := backupProviderTypeNames[int32(p)]; ok {
		return n
	}
	return "unknown"
}

func normalizeSeverity(s backup_managerpb.BackupSeverity) string {
	if n, ok := backupSeverityNames[int32(s)]; ok {
		return n
	}
	return "unknown"
}

// ── Helper: normalize a BackupJob ───────────────────────────────────────────

func normalizeJob(j *backup_managerpb.BackupJob) map[string]interface{} {
	if j == nil {
		return nil
	}
	dur := ""
	if j.GetFinishedUnixMs() > 0 && j.GetStartedUnixMs() > 0 {
		dur = fmtDuration(float64(j.GetFinishedUnixMs()-j.GetStartedUnixMs()) / 1000.0)
	}
	return map[string]interface{}{
		"job_id":      j.GetJobId(),
		"job_type":    normalizeJobType(j.GetJobType()),
		"state":       normalizeJobState(j.GetState()),
		"created_at":  fmtTime(j.GetCreatedUnixMs()),
		"started_at":  fmtTime(j.GetStartedUnixMs()),
		"finished_at": fmtTime(j.GetFinishedUnixMs()),
		"duration":    dur,
		"plan_name":   j.GetPlanName(),
		"backup_id":   j.GetBackupId(),
		"message":     j.GetMessage(),
	}
}

// ── Helper: normalize a BackupArtifact ──────────────────────────────────────

func normalizeArtifact(a *backup_managerpb.BackupArtifact) map[string]interface{} {
	if a == nil {
		return nil
	}

	provSummary := make([]map[string]interface{}, 0, len(a.GetProviderResults()))
	for _, pr := range a.GetProviderResults() {
		provSummary = append(provSummary, map[string]interface{}{
			"type":    normalizeProviderType(pr.GetType()),
			"state":   normalizeJobState(pr.GetState()),
			"summary": pr.GetSummary(),
			"size":    fmtBytes(pr.GetBytesWritten()),
		})
	}

	return map[string]interface{}{
		"backup_id":        a.GetBackupId(),
		"plan_name":        a.GetPlanName(),
		"created_at":       fmtTime(a.GetCreatedUnixMs()),
		"total_size":       fmtBytes(a.GetTotalBytes()),
		"quality_state":    normalizeQualityState(a.GetQualityState()),
		"mode":             normalizeBackupMode(a.GetMode()),
		"provider_summary": provSummary,
		"location":         a.GetLocation(),
		"cluster_id":       a.GetClusterId(),
		"domain":           a.GetDomain(),
	}
}

// ── Helper: normalize a BackupProviderResult ────────────────────────────────

func normalizeProviderResult(pr *backup_managerpb.BackupProviderResult) map[string]interface{} {
	if pr == nil {
		return nil
	}
	return map[string]interface{}{
		"type":          normalizeProviderType(pr.GetType()),
		"enabled":       pr.GetEnabled(),
		"state":         normalizeJobState(pr.GetState()),
		"severity":      normalizeSeverity(pr.GetSeverity()),
		"summary":       pr.GetSummary(),
		"error_message": pr.GetErrorMessage(),
		"started_at":    fmtTime(pr.GetStartedUnixMs()),
		"finished_at":   fmtTime(pr.GetFinishedUnixMs()),
		"bytes_written": fmtBytes(pr.GetBytesWritten()),
	}
}

func registerBackupTools(s *server) {

	// ── backup_list_jobs ────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_list_jobs",
		Description: "Lists recent backup jobs (backup, restore, retention) with their state, timing, and plan name. Supports filtering by state. Use this to check if backups are running, recently succeeded, or failed.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"state": {Type: "string", Description: "Filter by job state: queued, running, succeeded, failed, canceled (omit for all)", Enum: []string{"queued", "running", "succeeded", "failed", "canceled"}},
				"limit": {Type: "integer", Description: "Maximum number of jobs to return (default 20)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &backup_managerpb.ListBackupJobsRequest{
			Limit: uint32(getInt(args, "limit", 20)),
		}

		// Map state filter string to enum.
		if stateStr := getStr(args, "state"); stateStr != "" {
			stateMap := map[string]backup_managerpb.BackupJobState{
				"queued":    backup_managerpb.BackupJobState_BACKUP_JOB_QUEUED,
				"running":   backup_managerpb.BackupJobState_BACKUP_JOB_RUNNING,
				"succeeded": backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED,
				"failed":    backup_managerpb.BackupJobState_BACKUP_JOB_FAILED,
				"canceled":  backup_managerpb.BackupJobState_BACKUP_JOB_CANCELED,
			}
			if v, ok := stateMap[stateStr]; ok {
				req.State = v
			}
		}

		resp, err := client.ListBackupJobs(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("ListBackupJobs: %w", err)
		}

		jobs := make([]map[string]interface{}, 0, len(resp.GetJobs()))
		for _, j := range resp.GetJobs() {
			jobs = append(jobs, normalizeJob(j))
		}

		return map[string]interface{}{
			"total": resp.GetTotal(),
			"jobs":  jobs,
		}, nil
	})

	// ── backup_get_job ──────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_get_job",
		Description: "Returns full detail for a specific backup job including provider-level results, timing, and error messages. Use this to drill into a failed or running job.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"job_id": {Type: "string", Description: "The job ID to inspect"},
			},
			Required: []string{"job_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		jobID := getStr(args, "job_id")
		if jobID == "" {
			return nil, fmt.Errorf("job_id is required")
		}

		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetBackupJob(callCtx, &backup_managerpb.GetBackupJobRequest{
			JobId: jobID,
		})
		if err != nil {
			return nil, fmt.Errorf("GetBackupJob: %w", err)
		}

		j := resp.GetJob()
		if j == nil {
			return map[string]interface{}{"error": "job not found"}, nil
		}

		result := normalizeJob(j)

		// Add provider results.
		provResults := make([]map[string]interface{}, 0, len(j.GetResults()))
		for _, pr := range j.GetResults() {
			provResults = append(provResults, normalizeProviderResult(pr))
		}
		result["provider_results"] = provResults

		return result, nil
	})

	// ── backup_list_backups ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_list_backups",
		Description: "Lists completed backup artifacts with size, quality state, mode, and provider summary. Supports filtering by mode (service/cluster). Use this to see what backups are available for restore or validation.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"mode":  {Type: "string", Description: "Filter by backup mode: service or cluster (omit for all)", Enum: []string{"service", "cluster"}},
				"limit": {Type: "integer", Description: "Maximum number of backups to return (default 20)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &backup_managerpb.ListBackupsRequest{
			Limit: uint32(getInt(args, "limit", 20)),
		}

		if modeStr := getStr(args, "mode"); modeStr != "" {
			modeMap := map[string]backup_managerpb.BackupMode{
				"service": backup_managerpb.BackupMode_BACKUP_MODE_SERVICE,
				"cluster": backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER,
			}
			if v, ok := modeMap[modeStr]; ok {
				req.Mode = v
			}
		}

		resp, err := client.ListBackups(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("ListBackups: %w", err)
		}

		backups := make([]map[string]interface{}, 0, len(resp.GetBackups()))
		for _, b := range resp.GetBackups() {
			backups = append(backups, normalizeArtifact(b))
		}

		return map[string]interface{}{
			"total":   resp.GetTotal(),
			"backups": backups,
		}, nil
	})

	// ── backup_get_backup ───────────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_get_backup",
		Description: "Returns full detail for a specific backup artifact including all provider results, locations, quality state, and labels. Use this to inspect a backup before validating or restoring.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"backup_id": {Type: "string", Description: "The backup ID to inspect"},
			},
			Required: []string{"backup_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		backupID := getStr(args, "backup_id")
		if backupID == "" {
			return nil, fmt.Errorf("backup_id is required")
		}

		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetBackup(callCtx, &backup_managerpb.GetBackupRequest{
			BackupId: backupID,
		})
		if err != nil {
			return nil, fmt.Errorf("GetBackup: %w", err)
		}

		b := resp.GetBackup()
		if b == nil {
			return map[string]interface{}{"error": "backup not found"}, nil
		}

		result := normalizeArtifact(b)

		// Add full provider results.
		provResults := make([]map[string]interface{}, 0, len(b.GetProviderResults()))
		for _, pr := range b.GetProviderResults() {
			provResults = append(provResults, normalizeProviderResult(pr))
		}
		result["provider_results"] = provResults
		result["locations"] = b.GetLocations()
		result["labels"] = b.GetLabels()
		result["manifest_sha256"] = b.GetManifestSha256()
		result["schema_version"] = b.GetSchemaVersion()
		result["created_by"] = b.GetCreatedBy()

		return result, nil
	})

	// ── backup_validate_backup ──────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_validate_backup",
		Description: "Validates a backup artifact's integrity. Returns whether the backup is valid and any issues found. Set deep=true for thorough content verification (slower).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"backup_id": {Type: "string", Description: "The backup ID to validate"},
				"deep":      {Type: "boolean", Description: "If true, perform deep content verification (default false)"},
			},
			Required: []string{"backup_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		backupID := getStr(args, "backup_id")
		if backupID == "" {
			return nil, fmt.Errorf("backup_id is required")
		}

		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 60*time.Second)
		defer cancel()

		resp, err := client.ValidateBackup(callCtx, &backup_managerpb.ValidateBackupRequest{
			BackupId: backupID,
			Deep:     getBool(args, "deep", false),
		})
		if err != nil {
			return nil, fmt.Errorf("ValidateBackup: %w", err)
		}

		issues := make([]map[string]interface{}, 0, len(resp.GetIssues()))
		for _, issue := range resp.GetIssues() {
			issues = append(issues, map[string]interface{}{
				"severity": normalizeSeverity(issue.GetSeverity()),
				"message":  issue.GetMessage(),
			})
		}

		return map[string]interface{}{
			"valid":  resp.GetValid(),
			"issues": issues,
		}, nil
	})

	// ── backup_get_retention_status ─────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_get_retention_status",
		Description: "Returns the current retention policy configuration and stats: how many backups are kept, total size, and age range. Use this to verify retention is configured correctly.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetRetentionStatus(callCtx, &backup_managerpb.GetRetentionStatusRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetRetentionStatus: %w", err)
		}

		var policy map[string]interface{}
		if p := resp.GetPolicy(); p != nil {
			policy = map[string]interface{}{
				"keep_last_n":                p.GetKeepLastN(),
				"keep_days":                  p.GetKeepDays(),
				"max_total_bytes":            fmtBytes(p.GetMaxTotalBytes()),
				"min_restore_tested_to_keep": p.GetMinRestoreTestedToKeep(),
			}
		}

		return map[string]interface{}{
			"policy":               policy,
			"current_backup_count": resp.GetCurrentBackupCount(),
			"current_total_size":   fmtBytes(resp.GetCurrentTotalBytes()),
			"oldest_backup_at":     fmtTime(resp.GetOldestBackupUnixMs()),
			"newest_backup_at":     fmtTime(resp.GetNewestBackupUnixMs()),
		}, nil
	})

	// ── backup_preflight_check ──────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_preflight_check",
		Description: "Checks if all backup provider tools (etcdctl, restic, rclone, sctool) are installed and accessible. Use this to verify the backup system prerequisites before running a backup.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.PreflightCheck(callCtx, &backup_managerpb.PreflightCheckRequest{})
		if err != nil {
			return nil, fmt.Errorf("PreflightCheck: %w", err)
		}

		tools := make([]map[string]interface{}, 0, len(resp.GetTools()))
		for _, t := range resp.GetTools() {
			tools = append(tools, map[string]interface{}{
				"name":      t.GetName(),
				"installed": t.GetAvailable(),
				"version":   t.GetVersion(),
			})
		}

		return map[string]interface{}{
			"all_ok": resp.GetAllOk(),
			"tools":  tools,
		}, nil
	})

	// ── backup_get_schedule_status ───────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_get_schedule_status",
		Description: "Returns the current automatic backup schedule configuration: whether it is enabled, the configured interval, and when the next backup will fire.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetScheduleStatus(callCtx, &backup_managerpb.GetScheduleStatusRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetScheduleStatus: %w", err)
		}

		return map[string]interface{}{
			"enabled":     resp.GetEnabled(),
			"interval":    resp.GetInterval(),
			"next_fire_at": fmtTime(resp.GetNextFireUnixMs()),
		}, nil
	})

	// ── backup_get_recovery_status ──────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_get_recovery_status",
		Description: "Returns the recovery seed status: whether a seed file is present, destination is configured, credentials are available, and details about the last backup. Use this to verify disaster recovery readiness.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetRecoveryStatus(callCtx, &backup_managerpb.GetRecoveryStatusRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetRecoveryStatus: %w", err)
		}

		result := map[string]interface{}{
			"seed_present":           resp.GetSeedPresent(),
			"destination_configured": resp.GetDestinationConfigured(),
			"credentials_available":  resp.GetCredentialsAvailable(),
			"seed_matches_config":    resp.GetSeedMatchesConfig(),
			"cluster_name":           resp.GetClusterName(),
			"cluster_id":             resp.GetClusterId(),
			"domain":                 resp.GetDomain(),
			"seed_version":           resp.GetSeedVersion(),
			"message":                resp.GetMessage(),
		}

		if dest := resp.GetDestination(); dest != nil {
			result["destination"] = map[string]interface{}{
				"name": dest.GetName(),
				"type": dest.GetType(),
				"path": dest.GetPath(),
			}
		}

		if lb := resp.GetLastBackup(); lb != nil {
			result["last_backup"] = map[string]interface{}{
				"backup_id":     lb.GetBackupId(),
				"created_at":    fmtTime(lb.GetCreatedUnixMs()),
				"plan_name":     lb.GetPlanName(),
				"quality_state": lb.GetQualityState(),
			}
		}

		return result, nil
	})

	// ── backup_restore_plan ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_restore_plan",
		Description: "Generates a read-only preview of what a restore would do for a given backup. Shows the ordered steps and any warnings. Does NOT execute the restore. Use this to understand the impact before committing to a restore.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"backup_id":            {Type: "string", Description: "The backup ID to plan restore for"},
				"include_etcd":         {Type: "boolean", Description: "Include etcd restore (default false)"},
				"include_config":       {Type: "boolean", Description: "Include config restore (default false)"},
				"include_minio":        {Type: "boolean", Description: "Include MinIO restore (default false)"},
				"include_scylla":       {Type: "boolean", Description: "Include ScyllaDB restore (default false)"},
				"include_service_data": {Type: "boolean", Description: "Include service-declared local data restore (default false)"},
			},
			Required: []string{"backup_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		backupID := getStr(args, "backup_id")
		if backupID == "" {
			return nil, fmt.Errorf("backup_id is required")
		}

		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.RestorePlan(callCtx, &backup_managerpb.RestorePlanRequest{
			BackupId:           backupID,
			IncludeEtcd:        getBool(args, "include_etcd", false),
			IncludeConfig:      getBool(args, "include_config", false),
			IncludeMinio:       getBool(args, "include_minio", false),
			IncludeScylla:      getBool(args, "include_scylla", false),
			IncludeServiceData: getBool(args, "include_service_data", false),
		})
		if err != nil {
			return nil, fmt.Errorf("RestorePlan: %w", err)
		}

		steps := make([]map[string]interface{}, 0, len(resp.GetSteps()))
		for _, step := range resp.GetSteps() {
			steps = append(steps, map[string]interface{}{
				"order":   step.GetOrder(),
				"title":   step.GetTitle(),
				"details": step.GetDetails(),
			})
		}

		warnings := make([]map[string]interface{}, 0, len(resp.GetWarnings()))
		for _, w := range resp.GetWarnings() {
			warnings = append(warnings, map[string]interface{}{
				"severity": normalizeSeverity(w.GetSeverity()),
				"message":  w.GetMessage(),
			})
		}

		return map[string]interface{}{
			"backup_id": resp.GetBackupId(),
			"steps":     steps,
			"warnings":  warnings,
		}, nil
	})

	// ── backup_list_minio_buckets ────────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_list_minio_buckets",
		Description: "Lists all MinIO buckets on the configured endpoint, including creation date, size, and object count. Use this to verify backup storage is available.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.ListMinioBuckets(callCtx, &backup_managerpb.ListMinioBucketsRequest{})
		if err != nil {
			return nil, fmt.Errorf("ListMinioBuckets: %w", err)
		}

		buckets := make([]map[string]interface{}, 0, len(resp.GetBuckets()))
		for _, b := range resp.GetBuckets() {
			buckets = append(buckets, map[string]interface{}{
				"name":          b.GetName(),
				"creation_date": b.GetCreationDate(),
				"size":          fmtBytes(b.GetSizeBytes()),
				"object_count":  b.GetObjectCount(),
			})
		}

		return map[string]interface{}{
			"endpoint": resp.GetEndpoint(),
			"buckets":  buckets,
		}, nil
	})

	// ── backup_test_scylla_connection ────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_test_scylla_connection",
		Description: "Tests ScyllaDB connectivity including scylla-manager, agent reachability, cluster status, and storage location. Returns per-check results with suggested fixes for failures.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, backupManagerEndpoint())
		if err != nil {
			return nil, err
		}
		client := backup_managerpb.NewBackupManagerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.TestScyllaConnection(callCtx, &backup_managerpb.TestScyllaConnectionRequest{})
		if err != nil {
			return nil, fmt.Errorf("TestScyllaConnection: %w", err)
		}

		checks := make([]map[string]interface{}, 0, len(resp.GetChecks()))
		for _, c := range resp.GetChecks() {
			check := map[string]interface{}{
				"name":    c.GetName(),
				"ok":      c.GetOk(),
				"message": c.GetMessage(),
			}
			if fix := c.GetFix(); fix != "" {
				check["fix"] = fix
			}
			checks = append(checks, check)
		}

		return map[string]interface{}{
			"all_ok": resp.GetAllOk(),
			"checks": checks,
		}, nil
	})
}

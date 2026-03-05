package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"github.com/globulario/services/golang/config"
)

const defaultBackupManagerPort = 10040

func resolveBackupAddr() string {
	return config.ResolveServiceAddr(
		"backup_manager.BackupManagerService",
		fmt.Sprintf("localhost:%d", defaultBackupManagerPort),
	)
}

// ---- Variables for flags ----

var (
	backupCmd = &cobra.Command{
		Use:   "backup",
		Short: "Manage cluster backups",
	}

	// create flags
	backupCreateMode      string
	backupCreateProviders []string
	backupCreateServices  []string
	backupCreateLabels    []string
	backupCreatePlanName  string
	backupCreateWait      bool

	backupCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new backup",
		Long:  `Trigger a backup job. By default runs a CLUSTER-mode backup with all default providers.`,
		RunE:  runBackupCreate,
	}

	// list flags
	backupListLimit   uint32
	backupListOffset  uint32
	backupListMode    string
	backupListPlan    string
	backupListQuality string

	backupListCmd = &cobra.Command{
		Use:   "list",
		Short: "List completed backups",
		RunE:  runBackupList,
	}

	// restore flags
	backupRestoreID      string
	backupRestoreEtcd    bool
	backupRestoreConfig  bool
	backupRestoreMinio   bool
	backupRestoreScylla  bool
	backupRestoreDryRun  bool
	backupRestoreForce   bool
	backupRestoreAll     bool

	backupRestoreCmd = &cobra.Command{
		Use:   "restore",
		Short: "Restore from a backup",
		Long: `Two-phase restore: first generates a restore plan showing what will happen,
then executes it after confirmation. Use --dry-run to preview only.
Use --force to skip the confirmation token requirement.`,
		RunE: runBackupRestore,
	}
)

func init() {
	// create flags
	backupCreateCmd.Flags().StringVar(&backupCreateMode, "mode", "cluster", "Backup mode: cluster or service")
	backupCreateCmd.Flags().StringSliceVar(&backupCreateProviders, "provider", nil, "Providers to include (repeatable; empty = all defaults)")
	backupCreateCmd.Flags().StringSliceVar(&backupCreateServices, "service", nil, "Services to include in scope (repeatable)")
	backupCreateCmd.Flags().StringSliceVar(&backupCreateLabels, "label", nil, "Labels as key=value (repeatable)")
	backupCreateCmd.Flags().StringVar(&backupCreatePlanName, "plan", "", "Plan name (optional)")
	backupCreateCmd.Flags().BoolVar(&backupCreateWait, "wait", false, "Wait for backup to complete and show result")

	// list flags
	backupListCmd.Flags().Uint32Var(&backupListLimit, "limit", 20, "Maximum number of backups to return")
	backupListCmd.Flags().Uint32Var(&backupListOffset, "offset", 0, "Offset for pagination")
	backupListCmd.Flags().StringVar(&backupListMode, "mode", "", "Filter by mode: cluster, service, or all (default)")
	backupListCmd.Flags().StringVar(&backupListPlan, "plan", "", "Filter by plan name")
	backupListCmd.Flags().StringVar(&backupListQuality, "quality", "", "Filter by quality state: unverified, validated, restore-tested, promoted")

	// restore flags
	backupRestoreCmd.Flags().StringVar(&backupRestoreID, "backup-id", "", "Backup ID to restore (required)")
	backupRestoreCmd.Flags().BoolVar(&backupRestoreEtcd, "etcd", false, "Include etcd restore")
	backupRestoreCmd.Flags().BoolVar(&backupRestoreConfig, "config", false, "Include config restore")
	backupRestoreCmd.Flags().BoolVar(&backupRestoreMinio, "minio", false, "Include MinIO restore")
	backupRestoreCmd.Flags().BoolVar(&backupRestoreScylla, "scylla", false, "Include ScyllaDB restore")
	backupRestoreCmd.Flags().BoolVar(&backupRestoreAll, "all", false, "Include all providers in restore")
	backupRestoreCmd.Flags().BoolVar(&backupRestoreDryRun, "dry-run", false, "Preview restore plan without executing")
	backupRestoreCmd.Flags().BoolVar(&backupRestoreForce, "force", false, "Skip confirmation token requirement")

	backupCmd.AddCommand(backupCreateCmd, backupListCmd, backupRestoreCmd)
	rootCmd.AddCommand(backupCmd)
}

// ---- Create ----

func runBackupCreate(cmd *cobra.Command, args []string) error {
	addr := resolveBackupAddr()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("dial backup-manager at %s: %w", addr, err)
	}
	defer cc.Close()

	mode, err := parseBackupMode(backupCreateMode)
	if err != nil {
		return err
	}

	labels := parseLabels(backupCreateLabels)

	rqst := &backup_managerpb.RunBackupRequest{
		Mode:   mode,
		Labels: labels,
	}

	if backupCreatePlanName != "" || len(backupCreateProviders) > 0 {
		rqst.Plan = &backup_managerpb.BackupPlan{
			Name: backupCreatePlanName,
		}
	}

	if len(backupCreateProviders) > 0 || len(backupCreateServices) > 0 {
		rqst.Scope = &backup_managerpb.BackupScope{
			Providers: backupCreateProviders,
			Services:  backupCreateServices,
		}
	}

	ctx := ctxWithTimeout()
	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.RunBackup(ctx, rqst)
	if err != nil {
		return fmt.Errorf("RunBackup: %w", err)
	}

	if !backupCreateWait {
		fmt.Printf("Backup job started: %s\n", resp.JobId)
		return nil
	}

	// Poll until job completes
	return waitForJob(client, resp.JobId)
}

func waitForJob(client backup_managerpb.BackupManagerServiceClient, jobID string) error {
	fmt.Printf("Waiting for job %s ...\n", jobID)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	deadline := time.Now().Add(rootCfg.timeout)
	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for job %s", jobID)
			}
			ctx := ctxWithTimeout()
			resp, err := client.GetBackupJob(ctx, &backup_managerpb.GetBackupJobRequest{JobId: jobID})
			if err != nil {
				return fmt.Errorf("GetBackupJob: %w", err)
			}
			job := resp.Job
			switch job.State {
			case backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED:
				fmt.Printf("Backup succeeded: %s\n", job.BackupId)
				printJobSummary(job)
				return nil
			case backup_managerpb.BackupJobState_BACKUP_JOB_FAILED:
				printJobSummary(job)
				return fmt.Errorf("backup failed: %s", job.Message)
			case backup_managerpb.BackupJobState_BACKUP_JOB_CANCELED:
				return fmt.Errorf("backup canceled")
			}
		}
	}
}

func printJobSummary(job *backup_managerpb.BackupJob) {
	for _, r := range job.Results {
		status := "OK"
		if r.State == backup_managerpb.BackupJobState_BACKUP_JOB_FAILED {
			status = "FAILED"
		}
		fmt.Printf("  %-10s %s  %s\n", r.Type.String(), status, r.Summary)
	}
}

// ---- List ----

func runBackupList(cmd *cobra.Command, args []string) error {
	addr := resolveBackupAddr()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("dial backup-manager at %s: %w", addr, err)
	}
	defer cc.Close()

	rqst := &backup_managerpb.ListBackupsRequest{
		Limit:    backupListLimit,
		Offset:   backupListOffset,
		PlanName: backupListPlan,
	}

	if backupListMode != "" {
		mode, err := parseBackupMode(backupListMode)
		if err != nil {
			return err
		}
		rqst.Mode = mode
	}

	if backupListQuality != "" {
		q, err := parseQualityState(backupListQuality)
		if err != nil {
			return err
		}
		rqst.QualityState = q
	}

	ctx := ctxWithTimeout()
	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.ListBackups(ctx, rqst)
	if err != nil {
		return fmt.Errorf("ListBackups: %w", err)
	}

	switch rootCfg.output {
	case "json":
		data, err := json.MarshalIndent(backupArtifactsToMaps(resp.Backups), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(backupArtifactsToMaps(resp.Backups))
		if err != nil {
			return err
		}
		fmt.Print(string(data))
	default:
		printBackupsTable(resp.Backups, resp.Total)
	}
	return nil
}

func printBackupsTable(backups []*backup_managerpb.BackupArtifact, total uint32) {
	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "BACKUP ID\tCREATED\tMODE\tPROVIDERS\tSIZE\tQUALITY")
	for _, b := range backups {
		created := time.UnixMilli(b.CreatedUnixMs).Format("2006-01-02 15:04")
		mode := strings.TrimPrefix(b.Mode.String(), "BACKUP_MODE_")
		providers := providerList(b.ProviderResults)
		size := humanBytes(b.TotalBytes)
		quality := strings.TrimPrefix(b.QualityState.String(), "QUALITY_")
		if quality == "STATE_UNSPECIFIED" {
			quality = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", b.BackupId, created, mode, providers, size, quality)
	}
	w.Flush()
	if total > uint32(len(backups)) {
		fmt.Printf("\nShowing %d of %d total backups.\n", len(backups), total)
	}
}

func providerList(results []*backup_managerpb.BackupProviderResult) string {
	var names []string
	for _, r := range results {
		name := strings.TrimPrefix(r.Type.String(), "BACKUP_PROVIDER_")
		if r.State == backup_managerpb.BackupJobState_BACKUP_JOB_FAILED {
			name += "(FAIL)"
		}
		names = append(names, strings.ToLower(name))
	}
	return strings.Join(names, ",")
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func backupArtifactsToMaps(backups []*backup_managerpb.BackupArtifact) []map[string]interface{} {
	var out []map[string]interface{}
	for _, b := range backups {
		m := map[string]interface{}{
			"backup_id":     b.BackupId,
			"created":       time.UnixMilli(b.CreatedUnixMs).Format(time.RFC3339),
			"mode":          strings.TrimPrefix(b.Mode.String(), "BACKUP_MODE_"),
			"plan_name":     b.PlanName,
			"total_bytes":   b.TotalBytes,
			"quality_state": strings.TrimPrefix(b.QualityState.String(), "QUALITY_"),
			"location":      b.Location,
			"providers":     providerList(b.ProviderResults),
		}
		if b.Cluster != nil {
			m["cluster_id"] = b.Cluster.ClusterId
			m["domain"] = b.Cluster.Domain
			m["node_id"] = b.Cluster.NodeId
		}
		if len(b.Labels) > 0 {
			m["labels"] = b.Labels
		}
		out = append(out, m)
	}
	return out
}

// ---- Restore ----

func runBackupRestore(cmd *cobra.Command, args []string) error {
	if backupRestoreID == "" {
		return errors.New("--backup-id is required")
	}

	addr := resolveBackupAddr()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("dial backup-manager at %s: %w", addr, err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)

	includeEtcd := backupRestoreEtcd || backupRestoreAll
	includeConfig := backupRestoreConfig || backupRestoreAll
	includeMinio := backupRestoreMinio || backupRestoreAll
	includeScylla := backupRestoreScylla || backupRestoreAll

	// If no specific provider flags set and not --all, default to all
	if !backupRestoreEtcd && !backupRestoreConfig && !backupRestoreMinio && !backupRestoreScylla && !backupRestoreAll {
		includeEtcd = true
		includeConfig = true
		includeMinio = true
		includeScylla = true
	}

	// Phase 1: Get restore plan
	ctx := ctxWithTimeout()
	planResp, err := client.RestorePlan(ctx, &backup_managerpb.RestorePlanRequest{
		BackupId:      backupRestoreID,
		IncludeEtcd:   includeEtcd,
		IncludeConfig: includeConfig,
		IncludeMinio:  includeMinio,
		IncludeScylla: includeScylla,
	})
	if err != nil {
		return fmt.Errorf("RestorePlan: %w", err)
	}

	// Print restore plan
	fmt.Printf("Restore plan for backup %s:\n\n", planResp.BackupId)
	for _, step := range planResp.Steps {
		fmt.Printf("  %d. %s\n", step.Order, step.Title)
		if step.Details != "" {
			fmt.Printf("     %s\n", step.Details)
		}
	}

	if len(planResp.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range planResp.Warnings {
			fmt.Printf("  [%s] %s: %s\n", w.Severity.String(), w.Code, w.Message)
		}
	}

	if backupRestoreDryRun {
		fmt.Println("\n(dry-run) No changes made.")
		return nil
	}

	// Phase 2: Execute restore
	fmt.Println()
	ctx = ctxWithTimeout()
	restoreResp, err := client.RestoreBackup(ctx, &backup_managerpb.RestoreBackupRequest{
		BackupId:          backupRestoreID,
		IncludeEtcd:       includeEtcd,
		IncludeConfig:     includeConfig,
		IncludeMinio:      includeMinio,
		IncludeScylla:     includeScylla,
		Force:             backupRestoreForce,
		ConfirmationToken: planResp.ConfirmationToken,
	})
	if err != nil {
		return fmt.Errorf("RestoreBackup: %w", err)
	}

	fmt.Printf("Restore job started: %s\n", restoreResp.JobId)

	if len(restoreResp.Warnings) > 0 {
		for _, w := range restoreResp.Warnings {
			fmt.Printf("  [%s] %s\n", w.Severity.String(), w.Message)
		}
	}

	return nil
}

// ---- Helpers ----

func parseBackupMode(s string) (backup_managerpb.BackupMode, error) {
	switch strings.ToLower(s) {
	case "cluster":
		return backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER, nil
	case "service":
		return backup_managerpb.BackupMode_BACKUP_MODE_SERVICE, nil
	case "", "unspecified":
		return backup_managerpb.BackupMode_BACKUP_MODE_UNSPECIFIED, nil
	default:
		return 0, fmt.Errorf("invalid backup mode %q (use cluster or service)", s)
	}
}

func parseQualityState(s string) (backup_managerpb.QualityState, error) {
	switch strings.ToLower(s) {
	case "unverified":
		return backup_managerpb.QualityState_QUALITY_UNVERIFIED, nil
	case "validated":
		return backup_managerpb.QualityState_QUALITY_VALIDATED, nil
	case "restore-tested", "restore_tested":
		return backup_managerpb.QualityState_QUALITY_RESTORE_TESTED, nil
	case "promoted":
		return backup_managerpb.QualityState_QUALITY_PROMOTED, nil
	default:
		return 0, fmt.Errorf("invalid quality state %q (use unverified, validated, restore-tested, promoted)", s)
	}
}

func parseLabels(pairs []string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			m[p] = ""
		} else {
			m[k] = v
		}
	}
	return m
}


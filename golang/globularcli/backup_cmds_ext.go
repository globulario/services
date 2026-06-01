package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// ---- Variables for flags ----

var (
	// backup get
	backupGetID string

	backupGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Show details for a specific backup",
		RunE:  runBackupGet,
	}

	// backup delete
	backupDeleteID                string
	backupDeleteProviderArtifacts bool

	backupDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a backup artifact",
		RunE:  runBackupDelete,
	}

	// backup validate
	backupValidateID   string
	backupValidateDeep bool

	backupValidateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate backup integrity",
		RunE:  runBackupValidate,
	}

	// backup promote / demote
	backupPromoteID string
	backupDemoteID  string

	backupPromoteCmd = &cobra.Command{
		Use:   "promote",
		Short: "Promote a backup to PROMOTED quality state (protects from retention cleanup)",
		RunE:  runBackupPromote,
	}

	backupDemoteCmd = &cobra.Command{
		Use:   "demote",
		Short: "Demote a backup from PROMOTED quality state",
		RunE:  runBackupDemote,
	}

	// backup test
	backupTestID     string
	backupTestLevel  string
	backupTestTarget string

	backupTestCmd = &cobra.Command{
		Use:   "test",
		Short: "Run a restore test against a backup",
		Long: `Run a restore test to verify a backup can actually be restored.
LIGHT level checks metadata only (fast). HEAVY performs an actual sandbox restore (thorough).`,
		RunE: runBackupTest,
	}

	// backup preflight
	backupPreflightCmd = &cobra.Command{
		Use:   "preflight",
		Short: "Check that all required backup tools are installed",
		RunE:  runBackupPreflight,
	}

	// backup jobs subcommands
	backupJobsCmd = &cobra.Command{
		Use:   "jobs",
		Short: "Manage backup jobs",
	}

	backupJobsListLimit  uint32
	backupJobsListOffset uint32
	backupJobsListState  string
	backupJobsListPlan   string

	backupJobsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List backup jobs",
		RunE:  runBackupJobsList,
	}

	backupJobsGetID string

	backupJobsGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Show details for a specific backup job",
		RunE:  runBackupJobsGet,
	}

	backupJobsCancelID string

	backupJobsCancelCmd = &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a running backup job",
		RunE:  runBackupJobsCancel,
	}

	backupJobsDeleteID        string
	backupJobsDeleteArtifacts bool

	backupJobsDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a backup job record",
		RunE:  runBackupJobsDelete,
	}

	// backup retention subcommands
	backupRetentionCmd = &cobra.Command{
		Use:   "retention",
		Short: "Manage backup retention policy",
	}

	backupRetentionStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show current retention status and policy",
		RunE:  runBackupRetentionStatus,
	}

	backupRetentionRunDryRun bool

	backupRetentionRunCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the retention policy (delete expired backups)",
		RunE:  runBackupRetentionRun,
	}

	// backup schedule subcommands
	backupScheduleCmd = &cobra.Command{
		Use:   "schedule",
		Short: "Show backup schedule information",
	}

	backupScheduleStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show the configured backup schedule and next fire time",
		RunE:  runBackupScheduleStatus,
	}

	// backup recovery subcommands
	backupRecoveryCmd = &cobra.Command{
		Use:   "recovery",
		Short: "Manage disaster recovery configuration",
	}

	backupRecoveryStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show disaster recovery readiness",
		RunE:  runBackupRecoveryStatus,
	}

	backupRecoverySeedForce bool

	backupRecoverySeedCmd = &cobra.Command{
		Use:   "seed",
		Short: "Apply a recovery seed (persist backup destination for disaster recovery)",
		RunE:  runBackupRecoverySeed,
	}

	// backup scylla subcommands
	backupScyllaCmd = &cobra.Command{
		Use:   "scylla",
		Short: "ScyllaDB backup management",
	}

	backupScyllaTestCluster  string
	backupScyllaTestLocation string

	backupScyllaTestCmd = &cobra.Command{
		Use:   "test",
		Short: "Test the ScyllaDB / scylla-manager connection stack",
		RunE:  runBackupScyllaTest,
	}

	// backup minio subcommands
	backupMinioCmd = &cobra.Command{
		Use:   "minio",
		Short: "MinIO bucket management",
	}

	backupMinioListCmd = &cobra.Command{
		Use:   "list",
		Short: "List MinIO buckets",
		RunE:  runBackupMinioList,
	}

	backupMinioCreateName      string
	backupMinioCreateSetBackup bool
	backupMinioCreateSetScylla bool

	backupMinioCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a MinIO bucket",
		RunE:  runBackupMinioCreate,
	}

	backupMinioDeleteName  string
	backupMinioDeleteForce bool

	backupMinioDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a MinIO bucket",
		RunE:  runBackupMinioDelete,
	}
)

func init() {
	// backup get
	backupGetCmd.Flags().StringVar(&backupGetID, "backup-id", "", "Backup ID to retrieve (required)")

	// backup delete
	backupDeleteCmd.Flags().StringVar(&backupDeleteID, "backup-id", "", "Backup ID to delete (required)")
	backupDeleteCmd.Flags().BoolVar(&backupDeleteProviderArtifacts, "delete-artifacts", false, "Also delete provider-level artifacts (restic snapshots, scylla snapshots, etc.)")

	// backup validate
	backupValidateCmd.Flags().StringVar(&backupValidateID, "backup-id", "", "Backup ID to validate (required)")
	backupValidateCmd.Flags().BoolVar(&backupValidateDeep, "deep", false, "Deep validation: decompress and verify all providers (slower)")

	// backup promote / demote
	backupPromoteCmd.Flags().StringVar(&backupPromoteID, "backup-id", "", "Backup ID to promote (required)")
	backupDemoteCmd.Flags().StringVar(&backupDemoteID, "backup-id", "", "Backup ID to demote (required)")

	// backup test
	backupTestCmd.Flags().StringVar(&backupTestID, "backup-id", "", "Backup ID to test (empty = latest unverified)")
	backupTestCmd.Flags().StringVar(&backupTestLevel, "level", "light", "Test level: light (metadata checks) or heavy (sandbox restore)")
	backupTestCmd.Flags().StringVar(&backupTestTarget, "target", "", "Sandbox directory for heavy restore (default: auto)")

	// backup jobs list
	backupJobsListCmd.Flags().Uint32Var(&backupJobsListLimit, "limit", 20, "Maximum number of jobs to return")
	backupJobsListCmd.Flags().Uint32Var(&backupJobsListOffset, "offset", 0, "Offset for pagination")
	backupJobsListCmd.Flags().StringVar(&backupJobsListState, "state", "", "Filter by state: queued, running, succeeded, failed, canceled")
	backupJobsListCmd.Flags().StringVar(&backupJobsListPlan, "plan", "", "Filter by plan name")

	// backup jobs get
	backupJobsGetCmd.Flags().StringVar(&backupJobsGetID, "job-id", "", "Job ID to retrieve (required)")

	// backup jobs cancel
	backupJobsCancelCmd.Flags().StringVar(&backupJobsCancelID, "job-id", "", "Job ID to cancel (required)")

	// backup jobs delete
	backupJobsDeleteCmd.Flags().StringVar(&backupJobsDeleteID, "job-id", "", "Job ID to delete (required)")
	backupJobsDeleteCmd.Flags().BoolVar(&backupJobsDeleteArtifacts, "delete-artifacts", false, "Also delete the backup artifact created by this job")

	// backup retention run
	backupRetentionRunCmd.Flags().BoolVar(&backupRetentionRunDryRun, "dry-run", false, "Preview what would be deleted without deleting")

	// backup recovery seed
	backupRecoverySeedCmd.Flags().BoolVar(&backupRecoverySeedForce, "force", false, "Apply even if current config has non-local destinations")

	// backup scylla test
	backupScyllaTestCmd.Flags().StringVar(&backupScyllaTestCluster, "cluster", "", "Cluster name (uses config default if empty)")
	backupScyllaTestCmd.Flags().StringVar(&backupScyllaTestLocation, "location", "", "S3 location to test (uses config default if empty)")

	// backup minio create
	backupMinioCreateCmd.Flags().StringVar(&backupMinioCreateName, "name", "", "Bucket name (required)")
	backupMinioCreateCmd.Flags().BoolVar(&backupMinioCreateSetBackup, "set-backup-destination", false, "Auto-add as a backup destination")
	backupMinioCreateCmd.Flags().BoolVar(&backupMinioCreateSetScylla, "set-scylla-location", false, "Also set as the ScyllaDB backup location")

	// backup minio delete
	backupMinioDeleteCmd.Flags().StringVar(&backupMinioDeleteName, "name", "", "Bucket name to delete (required)")
	backupMinioDeleteCmd.Flags().BoolVar(&backupMinioDeleteForce, "force", false, "Delete even if the bucket contains objects")

	// Assemble subcommand trees
	backupJobsCmd.AddCommand(backupJobsListCmd, backupJobsGetCmd, backupJobsCancelCmd, backupJobsDeleteCmd)
	backupRetentionCmd.AddCommand(backupRetentionStatusCmd, backupRetentionRunCmd)
	backupScheduleCmd.AddCommand(backupScheduleStatusCmd)
	backupRecoveryCmd.AddCommand(backupRecoveryStatusCmd, backupRecoverySeedCmd)
	backupScyllaCmd.AddCommand(backupScyllaTestCmd)
	backupMinioCmd.AddCommand(backupMinioListCmd, backupMinioCreateCmd, backupMinioDeleteCmd)

	backupCmd.AddCommand(
		backupGetCmd,
		backupDeleteCmd,
		backupValidateCmd,
		backupPromoteCmd,
		backupDemoteCmd,
		backupTestCmd,
		backupPreflightCmd,
		backupJobsCmd,
		backupRetentionCmd,
		backupScheduleCmd,
		backupRecoveryCmd,
		backupScyllaCmd,
		backupMinioCmd,
	)
}

// ---- backup get ----

func runBackupGet(cmd *cobra.Command, args []string) error {
	id := backupGetID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--backup-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.GetBackup(ctxWithTimeout(), &backup_managerpb.GetBackupRequest{BackupId: id})
	if err != nil {
		return fmt.Errorf("GetBackup: %w", err)
	}

	b := resp.Backup
	switch rootCfg.output {
	case "json":
		data, _ := json.MarshalIndent(backupArtifactToMap(b), "", "  ")
		fmt.Println(string(data))
	case "yaml":
		data, _ := yaml.Marshal(backupArtifactToMap(b))
		fmt.Print(string(data))
	default:
		printBackupDetail(b)
	}
	return nil
}

func printBackupDetail(b *backup_managerpb.BackupArtifact) {
	if b == nil {
		fmt.Println("Not found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	created := time.UnixMilli(b.CreatedUnixMs).Format("2006-01-02 15:04:05")
	fmt.Fprintf(w, "Backup ID:\t%s\n", b.BackupId)
	fmt.Fprintf(w, "Created:\t%s\n", created)
	fmt.Fprintf(w, "Mode:\t%s\n", strings.TrimPrefix(b.Mode.String(), "BACKUP_MODE_"))
	fmt.Fprintf(w, "Plan:\t%s\n", b.PlanName)
	fmt.Fprintf(w, "Quality:\t%s\n", strings.TrimPrefix(b.QualityState.String(), "QUALITY_"))
	fmt.Fprintf(w, "Total Size:\t%s\n", humanBytes(b.TotalBytes))
	fmt.Fprintf(w, "Location:\t%s\n", b.Location)
	if b.Cluster != nil {
		fmt.Fprintf(w, "Domain:\t%s\n", b.Cluster.Domain)
		fmt.Fprintf(w, "Node ID:\t%s\n", b.Cluster.NodeId)
	}
	if len(b.Labels) > 0 {
		var lparts []string
		for k, v := range b.Labels {
			lparts = append(lparts, k+"="+v)
		}
		fmt.Fprintf(w, "Labels:\t%s\n", strings.Join(lparts, ", "))
	}
	w.Flush()

	if len(b.ProviderResults) > 0 {
		fmt.Println("\nProvider Results:")
		pw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(pw, "  PROVIDER\tSTATUS\tSUMMARY")
		for _, r := range b.ProviderResults {
			state := "OK"
			if r.State == backup_managerpb.BackupJobState_BACKUP_JOB_FAILED {
				state = "FAILED"
			}
			name := strings.TrimPrefix(r.Type.String(), "BACKUP_PROVIDER_")
			fmt.Fprintf(pw, "  %s\t%s\t%s\n", strings.ToLower(name), state, r.Summary)
		}
		pw.Flush()
	}
}

func backupArtifactToMap(b *backup_managerpb.BackupArtifact) map[string]interface{} {
	if b == nil {
		return nil
	}
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
	return m
}

// ---- backup delete ----

func runBackupDelete(cmd *cobra.Command, args []string) error {
	id := backupDeleteID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--backup-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.DeleteBackup(ctxWithTimeout(), &backup_managerpb.DeleteBackupRequest{
		BackupId:                id,
		DeleteProviderArtifacts: backupDeleteProviderArtifacts,
	})
	if err != nil {
		return fmt.Errorf("DeleteBackup: %w", err)
	}

	if resp.Deleted {
		fmt.Printf("Deleted backup %s\n", id)
	} else {
		fmt.Printf("Not deleted: %s\n", resp.Message)
	}
	for _, r := range resp.ProviderResults {
		status := "OK"
		if !r.Ok {
			status = "FAILED"
		}
		fmt.Printf("  %-20s %s  %s\n", r.Target, status, r.Message)
	}
	return nil
}

// ---- backup validate ----

func runBackupValidate(cmd *cobra.Command, args []string) error {
	id := backupValidateID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--backup-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.ValidateBackup(ctxWithTimeout(), &backup_managerpb.ValidateBackupRequest{
		BackupId: id,
		Deep:     backupValidateDeep,
	})
	if err != nil {
		return fmt.Errorf("ValidateBackup: %w", err)
	}

	if resp.Valid {
		fmt.Printf("Backup %s is VALID\n", id)
	} else {
		fmt.Printf("Backup %s is INVALID\n", id)
	}
	for _, issue := range resp.Issues {
		sev := strings.TrimPrefix(issue.Severity.String(), "BACKUP_SEVERITY_")
		fmt.Printf("  [%s] %s: %s\n", sev, issue.Code, issue.Message)
	}
	return nil
}

// ---- backup promote ----

func runBackupPromote(cmd *cobra.Command, args []string) error {
	id := backupPromoteID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--backup-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.PromoteBackup(ctxWithTimeout(), &backup_managerpb.PromoteBackupRequest{BackupId: id})
	if err != nil {
		return fmt.Errorf("PromoteBackup: %w", err)
	}

	quality := strings.TrimPrefix(resp.QualityState.String(), "QUALITY_")
	fmt.Printf("Backup %s → %s\n", id, quality)
	if resp.Message != "" {
		fmt.Println(resp.Message)
	}
	return nil
}

// ---- backup demote ----

func runBackupDemote(cmd *cobra.Command, args []string) error {
	id := backupDemoteID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--backup-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.DemoteBackup(ctxWithTimeout(), &backup_managerpb.DemoteBackupRequest{BackupId: id})
	if err != nil {
		return fmt.Errorf("DemoteBackup: %w", err)
	}

	quality := strings.TrimPrefix(resp.QualityState.String(), "QUALITY_")
	fmt.Printf("Backup %s → %s\n", id, quality)
	if resp.Message != "" {
		fmt.Println(resp.Message)
	}
	return nil
}

// ---- backup test ----

func runBackupTest(cmd *cobra.Command, args []string) error {
	level, err := parseRestoreTestLevel(backupTestLevel)
	if err != nil {
		return err
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.RunRestoreTest(ctxWithTimeout(), &backup_managerpb.RunRestoreTestRequest{
		BackupId:   backupTestID,
		Level:      level,
		TargetRoot: backupTestTarget,
	})
	if err != nil {
		return fmt.Errorf("RunRestoreTest: %w", err)
	}

	lvl := strings.TrimPrefix(resp.Level.String(), "RESTORE_TEST_")
	fmt.Printf("Restore test started (job %s) for backup %s [%s]\n", resp.JobId, resp.BackupId, lvl)
	return nil
}

// ---- backup preflight ----

func runBackupPreflight(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.PreflightCheck(ctxWithTimeout(), &backup_managerpb.PreflightCheckRequest{})
	if err != nil {
		return fmt.Errorf("PreflightCheck: %w", err)
	}

	switch rootCfg.output {
	case "json":
		type toolRow struct {
			Name    string `json:"name"`
			OK      bool   `json:"ok"`
			Version string `json:"version,omitempty"`
			Path    string `json:"path,omitempty"`
			Error   string `json:"error,omitempty"`
		}
		var rows []toolRow
		for _, t := range resp.Tools {
			rows = append(rows, toolRow{
				Name:    t.Name,
				OK:      t.Available,
				Version: t.Version,
				Path:    t.Path,
				Error:   t.ErrorMessage,
			})
		}
		data, _ := json.MarshalIndent(map[string]interface{}{"all_ok": resp.AllOk, "tools": rows}, "", "  ")
		fmt.Println(string(data))
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "TOOL\tSTATUS\tVERSION\tPATH")
		for _, t := range resp.Tools {
			status := "OK"
			if !t.Available {
				status = "MISSING"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Name, status, t.Version, t.Path)
			if t.ErrorMessage != "" {
				fmt.Fprintf(w, "  └─ %s\n", t.ErrorMessage)
			}
		}
		w.Flush()
		if !resp.AllOk {
			return fmt.Errorf("preflight check failed: one or more required tools are missing")
		}
	}
	return nil
}

// ---- backup jobs list ----

func runBackupJobsList(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	rqst := &backup_managerpb.ListBackupJobsRequest{
		Limit:    backupJobsListLimit,
		Offset:   backupJobsListOffset,
		PlanName: backupJobsListPlan,
	}
	if backupJobsListState != "" {
		s, err := parseJobState(backupJobsListState)
		if err != nil {
			return err
		}
		rqst.State = s
	}

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.ListBackupJobs(ctxWithTimeout(), rqst)
	if err != nil {
		return fmt.Errorf("ListBackupJobs: %w", err)
	}

	switch rootCfg.output {
	case "json":
		data, _ := json.MarshalIndent(jobsToMaps(resp.Jobs), "", "  ")
		fmt.Println(string(data))
	case "yaml":
		data, _ := yaml.Marshal(jobsToMaps(resp.Jobs))
		fmt.Print(string(data))
	default:
		printJobsTable(resp.Jobs, resp.Total)
	}
	return nil
}

func printJobsTable(jobs []*backup_managerpb.BackupJob, total uint32) {
	if len(jobs) == 0 {
		fmt.Println("No jobs found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "JOB ID\tTYPE\tSTATUS\tSTARTED\tDURATION\tBACKUP ID")
	for _, j := range jobs {
		started := ""
		if j.StartedUnixMs > 0 {
			started = time.UnixMilli(j.StartedUnixMs).Format("2006-01-02 15:04")
		}
		dur := ""
		if j.FinishedUnixMs > 0 && j.StartedUnixMs > 0 {
			dur = (time.Duration(j.FinishedUnixMs-j.StartedUnixMs) * time.Millisecond).Round(time.Second).String()
		}
		state := strings.TrimPrefix(j.State.String(), "BACKUP_JOB_")
		jobType := strings.TrimPrefix(j.JobType.String(), "BACKUP_JOB_TYPE_")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", j.JobId, strings.ToLower(jobType), state, started, dur, j.BackupId)
	}
	w.Flush()
	if total > uint32(len(jobs)) {
		fmt.Printf("\nShowing %d of %d total jobs.\n", len(jobs), total)
	}
}

func jobsToMaps(jobs []*backup_managerpb.BackupJob) []map[string]interface{} {
	var out []map[string]interface{}
	for _, j := range jobs {
		m := map[string]interface{}{
			"job_id":    j.JobId,
			"state":     strings.TrimPrefix(j.State.String(), "BACKUP_JOB_"),
			"job_type":  strings.TrimPrefix(j.JobType.String(), "BACKUP_JOB_TYPE_"),
			"backup_id": j.BackupId,
			"message":   j.Message,
		}
		if j.StartedUnixMs > 0 {
			m["started"] = time.UnixMilli(j.StartedUnixMs).Format(time.RFC3339)
		}
		if j.FinishedUnixMs > 0 {
			m["finished"] = time.UnixMilli(j.FinishedUnixMs).Format(time.RFC3339)
		}
		out = append(out, m)
	}
	return out
}

// ---- backup jobs get ----

func runBackupJobsGet(cmd *cobra.Command, args []string) error {
	id := backupJobsGetID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--job-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.GetBackupJob(ctxWithTimeout(), &backup_managerpb.GetBackupJobRequest{JobId: id})
	if err != nil {
		return fmt.Errorf("GetBackupJob: %w", err)
	}

	switch rootCfg.output {
	case "json":
		data, _ := json.MarshalIndent(jobsToMaps([]*backup_managerpb.BackupJob{resp.Job}), "", "  ")
		fmt.Println(string(data))
	default:
		j := resp.Job
		state := strings.TrimPrefix(j.State.String(), "BACKUP_JOB_")
		jobType := strings.ToLower(strings.TrimPrefix(j.JobType.String(), "BACKUP_JOB_TYPE_"))
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "Job ID:\t%s\n", j.JobId)
		fmt.Fprintf(w, "Type:\t%s\n", jobType)
		fmt.Fprintf(w, "State:\t%s\n", state)
		fmt.Fprintf(w, "Backup ID:\t%s\n", j.BackupId)
		if j.StartedUnixMs > 0 {
			fmt.Fprintf(w, "Started:\t%s\n", time.UnixMilli(j.StartedUnixMs).Format("2006-01-02 15:04:05"))
		}
		if j.FinishedUnixMs > 0 {
			dur := time.Duration(j.FinishedUnixMs-j.StartedUnixMs) * time.Millisecond
			fmt.Fprintf(w, "Finished:\t%s  (%s)\n", time.UnixMilli(j.FinishedUnixMs).Format("2006-01-02 15:04:05"), dur.Round(time.Second))
		}
		if j.Message != "" {
			fmt.Fprintf(w, "Message:\t%s\n", j.Message)
		}
		w.Flush()
		if len(j.Results) > 0 {
			fmt.Println("\nProvider Results:")
			printJobSummary(j)
		}
	}
	return nil
}

// ---- backup jobs cancel ----

func runBackupJobsCancel(cmd *cobra.Command, args []string) error {
	id := backupJobsCancelID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--job-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.CancelBackupJob(ctxWithTimeout(), &backup_managerpb.CancelBackupJobRequest{JobId: id})
	if err != nil {
		return fmt.Errorf("CancelBackupJob: %w", err)
	}

	if resp.Canceled {
		fmt.Printf("Job %s canceled\n", id)
	} else {
		fmt.Printf("Not canceled: %s\n", resp.Message)
	}
	return nil
}

// ---- backup jobs delete ----

func runBackupJobsDelete(cmd *cobra.Command, args []string) error {
	id := backupJobsDeleteID
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errors.New("--job-id is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.DeleteBackupJob(ctxWithTimeout(), &backup_managerpb.DeleteBackupJobRequest{
		JobId:           id,
		DeleteArtifacts: backupJobsDeleteArtifacts,
	})
	if err != nil {
		return fmt.Errorf("DeleteBackupJob: %w", err)
	}

	if resp.Deleted {
		fmt.Printf("Job %s deleted\n", id)
	} else {
		fmt.Printf("Not deleted: %s\n", resp.Message)
	}
	return nil
}

// ---- backup retention status ----

func runBackupRetentionStatus(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.GetRetentionStatus(ctxWithTimeout(), &backup_managerpb.GetRetentionStatusRequest{})
	if err != nil {
		return fmt.Errorf("GetRetentionStatus: %w", err)
	}

	switch rootCfg.output {
	case "json":
		m := map[string]interface{}{
			"backup_count": resp.CurrentBackupCount,
			"total_bytes":  resp.CurrentTotalBytes,
			"total_size":   humanBytes(resp.CurrentTotalBytes),
		}
		if resp.OldestBackupUnixMs > 0 {
			m["oldest_backup"] = time.UnixMilli(resp.OldestBackupUnixMs).Format(time.RFC3339)
		}
		if resp.NewestBackupUnixMs > 0 {
			m["newest_backup"] = time.UnixMilli(resp.NewestBackupUnixMs).Format(time.RFC3339)
		}
		if resp.Policy != nil {
			m["policy"] = map[string]interface{}{
				"keep_last_n":             resp.Policy.KeepLastN,
				"keep_days":               resp.Policy.KeepDays,
				"max_total_bytes":         resp.Policy.MaxTotalBytes,
				"min_restore_tested_keep": resp.Policy.MinRestoreTestedToKeep,
			}
		}
		data, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(data))
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "Current backups:\t%d\n", resp.CurrentBackupCount)
		fmt.Fprintf(w, "Total size:\t%s\n", humanBytes(resp.CurrentTotalBytes))
		if resp.OldestBackupUnixMs > 0 {
			fmt.Fprintf(w, "Oldest:\t%s\n", time.UnixMilli(resp.OldestBackupUnixMs).Format("2006-01-02 15:04"))
		}
		if resp.NewestBackupUnixMs > 0 {
			fmt.Fprintf(w, "Newest:\t%s\n", time.UnixMilli(resp.NewestBackupUnixMs).Format("2006-01-02 15:04"))
		}
		if p := resp.Policy; p != nil {
			fmt.Fprintln(w, "\nRetention Policy:")
			if p.KeepLastN > 0 {
				fmt.Fprintf(w, "  Keep last N:\t%d\n", p.KeepLastN)
			}
			if p.KeepDays > 0 {
				fmt.Fprintf(w, "  Keep days:\t%d\n", p.KeepDays)
			}
			if p.MaxTotalBytes > 0 {
				fmt.Fprintf(w, "  Max total:\t%s\n", humanBytes(p.MaxTotalBytes))
			}
			if p.MinRestoreTestedToKeep > 0 {
				fmt.Fprintf(w, "  Min restore-tested:\t%d\n", p.MinRestoreTestedToKeep)
			}
		}
		w.Flush()
	}
	return nil
}

// ---- backup retention run ----

func runBackupRetentionRun(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.RunRetention(ctxWithTimeout(), &backup_managerpb.RunRetentionRequest{
		DryRun: backupRetentionRunDryRun,
	})
	if err != nil {
		return fmt.Errorf("RunRetention: %w", err)
	}

	if backupRetentionRunDryRun {
		fmt.Println("(dry-run) Would delete:")
	} else {
		fmt.Println("Deleted:")
	}
	for _, id := range resp.DeletedBackupIds {
		fmt.Printf("  - %s\n", id)
	}
	if len(resp.DeletedBackupIds) == 0 {
		fmt.Println("  (nothing to delete)")
	}
	if resp.Message != "" {
		fmt.Println(resp.Message)
	}
	return nil
}

// ---- backup schedule status ----

func runBackupScheduleStatus(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.GetScheduleStatus(ctxWithTimeout(), &backup_managerpb.GetScheduleStatusRequest{})
	if err != nil {
		return fmt.Errorf("GetScheduleStatus: %w", err)
	}

	switch rootCfg.output {
	case "json":
		m := map[string]interface{}{
			"enabled":  resp.Enabled,
			"interval": resp.Interval,
		}
		if resp.NextFireUnixMs > 0 {
			m["next_fire"] = time.UnixMilli(resp.NextFireUnixMs).Format(time.RFC3339)
		}
		data, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(data))
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		enabled := "no"
		if resp.Enabled {
			enabled = "yes"
		}
		fmt.Fprintf(w, "Enabled:\t%s\n", enabled)
		fmt.Fprintf(w, "Interval:\t%s\n", resp.Interval)
		if resp.NextFireUnixMs > 0 {
			fmt.Fprintf(w, "Next fire:\t%s\n", time.UnixMilli(resp.NextFireUnixMs).Format("2006-01-02 15:04:05"))
		}
		w.Flush()
	}
	return nil
}

// ---- backup recovery status ----

func runBackupRecoveryStatus(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.GetRecoveryStatus(ctxWithTimeout(), &backup_managerpb.GetRecoveryStatusRequest{})
	if err != nil {
		return fmt.Errorf("GetRecoveryStatus: %w", err)
	}

	switch rootCfg.output {
	case "json":
		m := map[string]interface{}{
			"seed_present":           resp.SeedPresent,
			"destination_configured": resp.DestinationConfigured,
			"credentials_available":  resp.CredentialsAvailable,
			"seed_matches_config":    resp.SeedMatchesConfig,
			"cluster_name":           resp.ClusterName,
			"domain":                 resp.Domain,
		}
		if resp.LastBackup != nil {
			m["last_backup"] = map[string]interface{}{
				"backup_id": resp.LastBackup.BackupId,
				"created":   time.UnixMilli(resp.LastBackup.CreatedUnixMs).Format(time.RFC3339),
				"quality":   resp.LastBackup.QualityState,
			}
		}
		data, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(data))
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		yesno := func(b bool) string {
			if b {
				return "yes"
			}
			return "no"
		}
		fmt.Fprintf(w, "Seed present:\t%s\n", yesno(resp.SeedPresent))
		fmt.Fprintf(w, "Destination configured:\t%s\n", yesno(resp.DestinationConfigured))
		fmt.Fprintf(w, "Credentials available:\t%s\n", yesno(resp.CredentialsAvailable))
		fmt.Fprintf(w, "Seed matches config:\t%s\n", yesno(resp.SeedMatchesConfig))
		if resp.Domain != "" {
			fmt.Fprintf(w, "Domain:\t%s\n", resp.Domain)
		}
		if resp.Destination != nil {
			fmt.Fprintf(w, "Destination:\t%s (%s) %s\n", resp.Destination.Name, resp.Destination.Type, resp.Destination.Path)
		}
		if resp.LastBackup != nil && resp.LastBackup.BackupId != "" {
			created := time.UnixMilli(resp.LastBackup.CreatedUnixMs).Format("2006-01-02 15:04")
			fmt.Fprintf(w, "Last backup:\t%s  %s  [%s]\n", resp.LastBackup.BackupId, created, resp.LastBackup.QualityState)
		}
		if resp.Message != "" {
			fmt.Fprintf(w, "Message:\t%s\n", resp.Message)
		}
		w.Flush()
	}
	return nil
}

// ---- backup recovery seed ----

func runBackupRecoverySeed(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.ApplyRecoverySeed(ctxWithTimeout(), &backup_managerpb.ApplyRecoverySeedRequest{
		Force: backupRecoverySeedForce,
	})
	if err != nil {
		return fmt.Errorf("ApplyRecoverySeed: %w", err)
	}

	if resp.Ok {
		fmt.Println("Recovery seed applied.")
		if resp.AppliedDestination != nil {
			fmt.Printf("Destination: %s (%s) %s\n",
				resp.AppliedDestination.Name,
				resp.AppliedDestination.Type,
				resp.AppliedDestination.Path)
		}
	} else {
		fmt.Printf("Failed: %s\n", resp.Message)
	}
	return nil
}

// ---- backup scylla test ----

func runBackupScyllaTest(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.TestScyllaConnection(ctxWithTimeout(), &backup_managerpb.TestScyllaConnectionRequest{
		Cluster:  backupScyllaTestCluster,
		Location: backupScyllaTestLocation,
	})
	if err != nil {
		return fmt.Errorf("TestScyllaConnection: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "CHECK\tSTATUS\tDETAIL")
	for _, c := range resp.Checks {
		status := "OK"
		if !c.Ok {
			status = "FAIL"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, status, c.Message)
		if !c.Ok && c.Fix != "" {
			for _, line := range strings.Split(c.Fix, "\n") {
				fmt.Fprintf(w, "\t\t  → %s\n", line)
			}
		}
	}
	w.Flush()

	if !resp.AllOk {
		return fmt.Errorf("ScyllaDB connection test failed")
	}
	return nil
}

// ---- backup minio list ----

func runBackupMinioList(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.ListMinioBuckets(ctxWithTimeout(), &backup_managerpb.ListMinioBucketsRequest{})
	if err != nil {
		return fmt.Errorf("ListMinioBuckets: %w", err)
	}

	switch rootCfg.output {
	case "json":
		type row struct {
			Name        string `json:"name"`
			Created     string `json:"created,omitempty"`
			Size        string `json:"size"`
			ObjectCount uint64 `json:"object_count"`
		}
		var rows []row
		for _, b := range resp.Buckets {
			rows = append(rows, row{
				Name:        b.Name,
				Created:     b.CreationDate,
				Size:        humanBytes(b.SizeBytes),
				ObjectCount: b.ObjectCount,
			})
		}
		data, _ := json.MarshalIndent(map[string]interface{}{"endpoint": resp.Endpoint, "buckets": rows}, "", "  ")
		fmt.Println(string(data))
	default:
		if len(resp.Buckets) == 0 {
			fmt.Println("No buckets found.")
			return nil
		}
		if resp.Endpoint != "" {
			fmt.Printf("Endpoint: %s\n\n", resp.Endpoint)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSIZE\tOBJECTS\tCREATED")
		for _, b := range resp.Buckets {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", b.Name, humanBytes(b.SizeBytes), b.ObjectCount, b.CreationDate)
		}
		w.Flush()
	}
	return nil
}

// ---- backup minio create ----

func runBackupMinioCreate(cmd *cobra.Command, args []string) error {
	name := backupMinioCreateName
	if name == "" && len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		return errors.New("--name is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.CreateMinioBucket(ctxWithTimeout(), &backup_managerpb.CreateMinioBucketRequest{
		Name:                  name,
		SetAsBackupDestination: backupMinioCreateSetBackup,
		SetAsScyllaLocation:   backupMinioCreateSetScylla,
	})
	if err != nil {
		return fmt.Errorf("CreateMinioBucket: %w", err)
	}

	if resp.Ok {
		fmt.Printf("Created bucket: %s\n", resp.BucketName)
		if resp.Message != "" {
			fmt.Println(resp.Message)
		}
	} else {
		return fmt.Errorf("create failed: %s", resp.Message)
	}
	return nil
}

// ---- backup minio delete ----

func runBackupMinioDelete(cmd *cobra.Command, args []string) error {
	name := backupMinioDeleteName
	if name == "" && len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		return errors.New("--name is required")
	}

	cc, err := dialGRPC(resolveBackupAddr())
	if err != nil {
		return fmt.Errorf("dial backup-manager: %w", err)
	}
	defer cc.Close()

	client := backup_managerpb.NewBackupManagerServiceClient(cc)
	resp, err := client.DeleteMinioBucket(ctxWithTimeout(), &backup_managerpb.DeleteMinioBucketRequest{
		Name:  name,
		Force: backupMinioDeleteForce,
	})
	if err != nil {
		return fmt.Errorf("DeleteMinioBucket: %w", err)
	}

	if resp.Ok {
		fmt.Printf("Deleted bucket: %s\n", name)
	} else {
		return fmt.Errorf("delete failed: %s", resp.Message)
	}
	return nil
}

// ---- Helpers ----

func parseJobState(s string) (backup_managerpb.BackupJobState, error) {
	switch strings.ToLower(s) {
	case "queued":
		return backup_managerpb.BackupJobState_BACKUP_JOB_QUEUED, nil
	case "running":
		return backup_managerpb.BackupJobState_BACKUP_JOB_RUNNING, nil
	case "succeeded":
		return backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED, nil
	case "failed":
		return backup_managerpb.BackupJobState_BACKUP_JOB_FAILED, nil
	case "canceled":
		return backup_managerpb.BackupJobState_BACKUP_JOB_CANCELED, nil
	default:
		return 0, fmt.Errorf("invalid job state %q (use queued, running, succeeded, failed, canceled)", s)
	}
}

func parseRestoreTestLevel(s string) (backup_managerpb.RestoreTestLevel, error) {
	switch strings.ToLower(s) {
	case "light", "":
		return backup_managerpb.RestoreTestLevel_RESTORE_TEST_LIGHT, nil
	case "heavy":
		return backup_managerpb.RestoreTestLevel_RESTORE_TEST_HEAVY, nil
	default:
		return 0, fmt.Errorf("invalid test level %q (use light or heavy)", s)
	}
}

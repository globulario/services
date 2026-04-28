// repo_upstream_cmds.go — Upstream source management for the repository.
//
// Commands:
//   globular repo register-upstream --name <n> --url <index-url> [--channel <c>] [--platform <p>] [--disabled]
//   globular repo list-upstreams
//   globular repo remove-upstream <name>
//   globular pkg sync-upstream --source <name> --tag <tag> [--dry-run] [--only a,b,c]
//
// Sync entrypoint:
//   pkg sync-upstream triggers the repository.sync.upstream workflow via the
//   centralized WorkflowService. The workflow executes three steps on the
//   repository actor (dispatched through the cluster-controller):
//     1. validate_source  — confirm source exists and is enabled
//     2. sync             — fetch index, verify digests, import artifacts
//     3. report           — emit structured audit outcome
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/repository/upstream"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/spf13/cobra"
)

// ── Flag variables ────────────────────────────────────────────────────────────

var (
	upstreamName     string
	upstreamURL      string
	upstreamChannel  string
	upstreamPlatform string
	upstreamDisabled bool

	// Policy flags for register-upstream
	upstreamPublisher       string
	upstreamAllowedKinds    string
	upstreamAllowedChannels string
	upstreamRequireChecksum bool
	upstreamTrustPolicy     string
	upstreamCredentialsRef  string
	upstreamRepoURL         string

	// Provider-specific flags
	upstreamType            string
	upstreamOwner           string
	upstreamRepo            string
	upstreamBranch          string
	upstreamIndexPath       string
	upstreamArtifactBaseURL string
	upstreamLocalRoot       string

	syncSource string
	syncTag    string
	syncDryRun bool
	syncOnly   string
	syncLatest bool
)

// ── Commands ──────────────────────────────────────────────────────────────────

var repoRegisterUpstreamCmd = &cobra.Command{
	Use:   "register-upstream",
	Short: "Register or update an upstream package source",
	Long: `Register a named upstream source so the cluster can sync new releases.

Supported types: github, http, local-dir, git (Phase 2).

The type is inferred from flags when --type is omitted:
  --repo-url or --owner → GITHUB_RELEASE
  --local-root          → LOCAL_DIR
  --index-url           → HTTP_INDEX`,
	Example: `  # Register the default globulario upstream (GitHub)
  globular repo register-upstream \
    --name globulario-github \
    --url "https://github.com/globulario/services/releases/download/{tag}/release-index.json"

  # Register with a specific channel and platform
  globular repo register-upstream \
    --name my-upstream \
    --url "https://example.com/releases/{tag}/index.json" \
    --channel stable \
    --platform linux_amd64`,
	RunE: runRepoRegisterUpstream,
}

var repoListUpstreamsCmd = &cobra.Command{
	Use:   "list-upstreams",
	Short: "List registered upstream sources",
	RunE:  runRepoListUpstreams,
}

var repoRemoveUpstreamCmd = &cobra.Command{
	Use:   "remove-upstream <name>",
	Short: "Remove a registered upstream source",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoRemoveUpstream,
}

// repoSyncCmd calls SyncFromUpstream directly on the repository service,
// bypassing the WorkflowService. Use this during bootstrap (Day-0) when the
// workflow service and cluster controller may not yet be registered in etcd.
// For Day-1+ operations, prefer `pkg sync-upstream` (workflow-tracked, audited).
var repoSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Import packages from a registered upstream release (direct, no workflow)",
	Long: `Sync packages from a registered upstream source directly via the
repository service. No WorkflowService or ClusterController needed.

Each artifact in the release index is verified by sha256 digest before
import. Existing artifacts with the same identity key are skipped.
Digest conflicts are counted as rejected (audit-logged, not imported).

Use --dry-run to preview what would be imported without writing anything.

This command is suitable for bootstrap (Day-0) and air-gapped environments
where the workflow infrastructure may not yet be available.
For audited Day-1 upgrades, use 'globular pkg sync-upstream' instead.`,
	Example: `  # Day-0 bootstrap sync (direct, no workflow)
  globular repo sync --source globulario-github --tag v1.0.27

  # Preview what would be imported
  globular repo sync --source globulario-github --tag v1.0.27 --dry-run

  # Import only specific packages
  globular repo sync --source globulario-github --tag v1.0.27 --only dns,rbac`,
	RunE: runRepoSync,
}

var pkgSyncUpstreamCmd = &cobra.Command{
	Use:   "sync-upstream",
	Short: "Import packages from a registered upstream release",
	Long: `Sync packages from a registered upstream source into the local repository.

Each artifact in the release index is verified by sha256 digest before
import. Existing artifacts with the same identity key are skipped.
Digest conflicts are counted as rejected (audit-logged, not imported).

Use --dry-run to preview what would be imported without writing anything.`,
	Example: `  # Preview what would be imported from v1.0.18
  globular pkg sync-upstream --source globulario-github --tag v1.0.18 --dry-run

  # Import all packages from v1.0.18
  globular pkg sync-upstream --source globulario-github --tag v1.0.18

  # Import only specific packages
  globular pkg sync-upstream --source globulario-github --tag v1.0.18 \
    --only dns,rbac,authentication`,
	RunE: runPkgSyncUpstream,
}

// ── Implementations ───────────────────────────────────────────────────────────

func repoClient() (*repository_client.Repository_Service_Client, error) {
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr == "" {
		if a, err := config.GetMeshAddress(); err == nil {
			addr = a
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("repository service not found (checked etcd and mesh)")
	}
	return repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
}

func runRepoRegisterUpstream(cmd *cobra.Command, args []string) error {
	if upstreamName == "" {
		return fmt.Errorf("--name is required")
	}

	// ── Infer provider type ──────────────────────────────────────────────
	sourceType := inferSourceType(cmd)

	// When GitHub: derive index_url from repo-url/owner/repo if not explicit.
	if sourceType == repopb.UpstreamSourceType_GITHUB_RELEASE {
		if upstreamURL == "" {
			owner := upstreamOwner
			repo := upstreamRepo
			if owner == "" && repo == "" && upstreamRepoURL != "" {
				var err error
				owner, repo, err = upstream.ParseRepoURL(upstreamRepoURL)
				if err != nil {
					return fmt.Errorf("invalid --repo-url: %w", err)
				}
			}
			if owner != "" && repo != "" {
				upstreamURL = upstream.DeriveIndexURL(owner, repo)
			}
		}
	}

	// Validate that we have enough to find an index.
	switch sourceType {
	case repopb.UpstreamSourceType_LOCAL_DIR:
		if upstreamLocalRoot == "" {
			return fmt.Errorf("--local-root is required for LOCAL_DIR sources")
		}
	default:
		if upstreamURL == "" && upstreamRepoURL == "" {
			return fmt.Errorf("--url, --repo-url, or --local-root is required")
		}
		if upstreamURL != "" && !strings.Contains(upstreamURL, "{tag}") {
			return fmt.Errorf("--url must contain a {tag} placeholder")
		}
	}

	platform := upstreamPlatform
	if platform == "" {
		platform = "linux_amd64"
	}
	channel := upstreamChannel
	if channel == "" {
		channel = "stable"
	}

	// Safe defaults for non-official sources.
	trustPolicy := upstreamTrustPolicy
	requireChecksum := upstreamRequireChecksum
	if !cmd.Flags().Changed("trust-policy") {
		trustPolicy = "quarantine"
	}
	if !cmd.Flags().Changed("require-checksum") {
		requireChecksum = true
	}

	if trustPolicy == "import" {
		fmt.Println("WARNING: trust_policy=import means imported packages are PUBLISHED immediately.")
		fmt.Println("         Use --trust-policy quarantine for untrusted sources.")
	}

	src := &repopb.UpstreamSource{
		Name:               upstreamName,
		Type:               sourceType,
		IndexUrl:           upstreamURL,
		Channel:            channel,
		Platform:           platform,
		Enabled:            !upstreamDisabled,
		DefaultPublisherId: upstreamPublisher,
		RequireChecksum:    requireChecksum,
		TrustPolicy:        trustPolicy,
		CredentialsRef:     upstreamCredentialsRef,
		RepoUrl:            upstreamRepoURL,
		Owner:              upstreamOwner,
		Repo:               upstreamRepo,
		Branch:             upstreamBranch,
		IndexPathTemplate:  upstreamIndexPath,
		ArtifactBaseUrl:    upstreamArtifactBaseURL,
		LocalRoot:          upstreamLocalRoot,
	}
	if upstreamAllowedKinds != "" {
		for _, k := range strings.Split(upstreamAllowedKinds, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				src.AllowedKinds = append(src.AllowedKinds, k)
			}
		}
	}
	if upstreamAllowedChannels != "" {
		for _, c := range strings.Split(upstreamAllowedChannels, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				src.AllowedChannels = append(src.AllowedChannels, c)
			}
		}
	} else {
		// Safe default: restrict to stable channel.
		src.AllowedChannels = []string{"stable"}
	}

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = rc.RegisterUpstream(src)
	if err != nil {
		return fmt.Errorf("register upstream: %w", err)
	}

	fmt.Printf("Upstream %q registered\n", upstreamName)
	fmt.Printf("  type:     %s\n", sourceType.String())
	fmt.Printf("  url:      %s\n", upstreamURL)
	fmt.Printf("  channel:  %s\n", channel)
	fmt.Printf("  platform: %s\n", platform)
	fmt.Printf("  enabled:  %v\n", !upstreamDisabled)
	return nil
}

func runRepoListUpstreams(cmd *cobra.Command, args []string) error {
	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	resp, err := rc.ListUpstreams()
	if err != nil {
		return fmt.Errorf("list upstreams: %w", err)
	}

	if len(resp.Sources) == 0 {
		fmt.Println("No upstream sources registered.")
		fmt.Println("Use 'globular repo register-upstream' to add one.")
		return nil
	}

	fmt.Printf("%-24s  %-12s  %-12s  %-12s  %-10s  %s\n",
		"NAME", "CHANNEL", "PLATFORM", "LAST_TAG", "ENABLED", "URL")
	fmt.Println(strings.Repeat("-", 100))
	for _, s := range resp.Sources {
		enabled := "yes"
		if !s.Enabled {
			enabled = "no"
		}
		lastTag := s.LastSyncedTag
		if lastTag == "" {
			lastTag = "-"
		}
		fmt.Printf("%-24s  %-12s  %-12s  %-12s  %-10s  %s\n",
			s.Name, s.Channel, s.Platform, lastTag, enabled, s.IndexUrl)
	}
	return nil
}

func runRepoRemoveUpstream(cmd *cobra.Command, args []string) error {
	name := args[0]

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	err = rc.RemoveUpstream(name)
	if err != nil {
		return fmt.Errorf("remove upstream: %w", err)
	}

	fmt.Printf("Upstream %q removed.\n", name)
	return nil
}

func runPkgSyncUpstream(cmd *cobra.Command, args []string) error {
	if syncSource == "" {
		return fmt.Errorf("--source is required")
	}
	if syncTag == "" {
		return fmt.Errorf("--tag is required")
	}

	var only []string
	if syncOnly != "" {
		for _, s := range strings.Split(syncOnly, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				only = append(only, s)
			}
		}
	}

	// Resolve workflow service address from etcd registry.
	wfAddr := config.ResolveServiceAddr("workflow.WorkflowService", "")
	if wfAddr == "" {
		if a, err := config.GetMeshAddress(); err == nil {
			wfAddr = a
		}
	}
	if wfAddr == "" {
		return fmt.Errorf("workflow service not found (check etcd or use --controller)")
	}

	// Resolve controller address for actor endpoint callbacks.
	// The workflow service dispatches repository actor actions to the controller,
	// which has the repository actor handler registered in its defaultRouter.
	controllerAddr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", "")
	if controllerAddr == "" {
		controllerAddr = rootCfg.controllerAddr
	}

	// Build workflow inputs.
	inputs := map[string]any{
		"source_name": syncSource,
		"release_tag": syncTag,
		"dry_run":     syncDryRun,
	}
	if len(only) > 0 {
		inputs["only"] = only
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}

	corrID := fmt.Sprintf("cli-sync-%s-%s", syncSource, syncTag)

	if syncDryRun {
		fmt.Printf("Dry-run: previewing sync from %q @ %s\n\n", syncSource, syncTag)
	} else {
		fmt.Printf("Syncing from %q @ %s via workflow...\n\n", syncSource, syncTag)
	}

	cc, err := dialGRPC(wfAddr)
	if err != nil {
		return fmt.Errorf("connect to workflow service at %s: %w", wfAddr, err)
	}
	defer cc.Close()

	wfClient := workflowpb.NewWorkflowServiceClient(cc)
	resp, err := wfClient.ExecuteWorkflow(ctxWithTimeout(), &workflowpb.ExecuteWorkflowRequest{
		WorkflowName: "repository.sync.upstream",
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"repository":         controllerAddr,
			"cluster-controller": controllerAddr,
		},
		CorrelationId: corrID,
	})
	if err != nil {
		return fmt.Errorf("workflow: %w", err)
	}

	fmt.Printf("Run ID: %s\n", resp.RunId)

	// Decode outputs from the workflow's accumulated step exports.
	if resp.OutputsJson != "" {
		var outputs map[string]any
		if jsonErr := json.Unmarshal([]byte(resp.OutputsJson), &outputs); jsonErr == nil {
			if summary, ok := outputs["sync_summary"].(map[string]any); ok {
				printSyncSummary(summary, syncDryRun)
			}
		}
	}

	if resp.Status == "FAILED" {
		return fmt.Errorf("sync workflow failed: %s", resp.Error)
	}
	return nil
}

func runRepoSync(cmd *cobra.Command, args []string) error {
	if syncSource == "" {
		return fmt.Errorf("--source is required")
	}
	if syncTag != "" && syncLatest {
		return fmt.Errorf("cannot use both --tag and --latest")
	}
	if syncTag == "" && !syncLatest {
		return fmt.Errorf("--tag or --latest is required")
	}

	var only []string
	if syncOnly != "" {
		for _, s := range strings.Split(syncOnly, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				only = append(only, s)
			}
		}
	}

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	tagDisplay := syncTag
	if syncLatest {
		tagDisplay = "(latest)"
	}
	if syncDryRun {
		fmt.Printf("Dry-run: previewing sync from %q @ %s\n\n", syncSource, tagDisplay)
	} else {
		fmt.Printf("Syncing from %q @ %s (direct)...\n\n", syncSource, tagDisplay)
	}

	resp, err := rc.SyncFromUpstreamWithOptions(&repopb.SyncFromUpstreamRequest{
		SourceName:    syncSource,
		ReleaseTag:    syncTag,
		DryRun:        syncDryRun,
		Only:          only,
		ResolveLatest: syncLatest,
	})
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	if resp.ResolvedTag != "" {
		fmt.Printf("Resolved tag: %s\n\n", resp.ResolvedTag)
	}

	summary := map[string]any{
		"imported": int64(resp.Imported),
		"skipped":  int64(resp.Skipped),
		"rejected": int64(resp.Rejected),
		"failed":   int64(resp.Failed),
	}
	printSyncSummary(summary, syncDryRun)
	return nil
}

func printSyncSummary(summary map[string]any, dryRun bool) {
	toI := func(v any) int64 {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int32:
			return int64(n)
		case int64:
			return n
		}
		return 0
	}

	imported := toI(summary["imported"])
	skipped := toI(summary["skipped"])
	rejected := toI(summary["rejected"])
	failed := toI(summary["failed"])

	if dryRun {
		fmt.Printf("Would import: %d   would skip: %d   would reject: %d   would fail: %d\n",
			imported, skipped, rejected, failed)
	} else {
		fmt.Printf("Imported: %d   skipped: %d   rejected: %d   failed: %d\n",
			imported, skipped, rejected, failed)
	}
}

// statusLabel is kept for potential future use with direct-repo commands.
func statusLabel(s repopb.UpstreamSyncStatus) string {
	switch s {
	case repopb.UpstreamSyncStatus_SYNC_IMPORTED:
		return "IMPORTED"
	case repopb.UpstreamSyncStatus_SYNC_SKIPPED:
		return "SKIPPED"
	case repopb.UpstreamSyncStatus_SYNC_REJECTED:
		return "REJECTED"
	case repopb.UpstreamSyncStatus_SYNC_FAILED:
		return "FAILED"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT:
		return "WOULD_IMPORT"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP:
		return "WOULD_SKIP"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT:
		return "WOULD_REJECT"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_FAIL:
		return "WOULD_FAIL"
	default:
		return "UNKNOWN"
	}
}

// inferSourceType determines the UpstreamSourceType from flags.
// Explicit --type wins; otherwise inferred from provider-specific flags.
func inferSourceType(cmd *cobra.Command) repopb.UpstreamSourceType {
	if cmd.Flags().Changed("type") {
		switch strings.ToLower(upstreamType) {
		case "github", "github_release":
			return repopb.UpstreamSourceType_GITHUB_RELEASE
		case "http", "http_index":
			return repopb.UpstreamSourceType_HTTP_INDEX
		case "local-dir", "local_dir":
			return repopb.UpstreamSourceType_LOCAL_DIR
		case "git", "git_index":
			return repopb.UpstreamSourceType_GIT_INDEX
		default:
			return repopb.UpstreamSourceType_UPSTREAM_TYPE_UNSPECIFIED
		}
	}
	// Infer from flags.
	if upstreamLocalRoot != "" {
		return repopb.UpstreamSourceType_LOCAL_DIR
	}
	if upstreamRepoURL != "" || upstreamOwner != "" {
		return repopb.UpstreamSourceType_GITHUB_RELEASE
	}
	if upstreamURL != "" {
		return repopb.UpstreamSourceType_HTTP_INDEX
	}
	return repopb.UpstreamSourceType_GITHUB_RELEASE // default
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// register-upstream flags
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamName, "name", "", "Upstream source name (required)")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamType, "type", "", "Provider type: github, http, local-dir, git")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamURL, "url", "", "Index URL with {tag} placeholder")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamChannel, "channel", "stable", "Release channel")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamPlatform, "platform", "linux_amd64", "Target platform")
	repoRegisterUpstreamCmd.Flags().BoolVar(&upstreamDisabled, "disabled", false, "Register but do not enable")
	// Policy flags
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamPublisher, "publisher", "", "Default publisher ID for entries without one")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamAllowedKinds, "allowed-kinds", "", "Comma-separated allowed kinds")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamAllowedChannels, "allowed-channels", "", "Comma-separated allowed channels")
	repoRegisterUpstreamCmd.Flags().BoolVar(&upstreamRequireChecksum, "require-checksum", false, "Reject entries without sha256 digest")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamTrustPolicy, "trust-policy", "import", "Trust policy: import or quarantine")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamCredentialsRef, "credentials-ref", "", "etcd key under /globular/credentials/ for auth")
	// Provider-specific flags
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamRepoURL, "repo-url", "", "Git repo URL or GitHub owner/repo")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamOwner, "owner", "", "GitHub owner (GITHUB_RELEASE)")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamRepo, "repo", "", "GitHub repo name (GITHUB_RELEASE)")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamBranch, "branch", "", "Git branch (GIT_INDEX)")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamIndexPath, "index-path", "", "Index path template: releases/{tag}/release-index.json")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamArtifactBaseURL, "artifact-base-url", "", "Base URL for artifact downloads")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamLocalRoot, "local-root", "", "Filesystem root for LOCAL_DIR sources")

	// sync-upstream flags
	pkgSyncUpstreamCmd.Flags().StringVar(&syncSource, "source", "", "Registered upstream source name (required)")
	pkgSyncUpstreamCmd.Flags().StringVar(&syncTag, "tag", "", "Release tag to sync")
	pkgSyncUpstreamCmd.Flags().BoolVar(&syncLatest, "latest", false, "Discover and sync latest release from repo_url")
	pkgSyncUpstreamCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Preview only — no artifacts are written")
	pkgSyncUpstreamCmd.Flags().StringVar(&syncOnly, "only", "", "Comma-separated list of package names to import")

	// repo sync flags
	repoSyncCmd.Flags().StringVar(&syncSource, "source", "", "Registered upstream source name (required)")
	repoSyncCmd.Flags().StringVar(&syncTag, "tag", "", "Release tag to sync")
	repoSyncCmd.Flags().BoolVar(&syncLatest, "latest", false, "Discover and sync latest release from repo_url")
	repoSyncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Preview only — no artifacts are written")
	repoSyncCmd.Flags().StringVar(&syncOnly, "only", "", "Comma-separated list of package names to import")

	repoCmd.AddCommand(repoRegisterUpstreamCmd)
	repoCmd.AddCommand(repoListUpstreamsCmd)
	repoCmd.AddCommand(repoRemoveUpstreamCmd)
	repoCmd.AddCommand(repoSyncCmd)

	pkgCmd.AddCommand(pkgSyncUpstreamCmd)
}

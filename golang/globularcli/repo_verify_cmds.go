package main

// repo_verify_cmds.go — `globular repository verify | repair | explain`.
//
// These commands are operator frontends to the repository service's
// VerifyArtifact / RepairArtifact / ExplainArtifact RPCs. The CLI never
// duplicates verification logic — the repository service is the single
// source of truth.

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Command declarations ───────────────────────────────────────────────────

var (
	verifyPlatform     string
	verifyBuildNumber  int64
	verifyBuildID      string
	verifyKind         string
	verifyDigestMode   string // "full" | "stat" | "none"
	verifySignature    bool
	verifyLedger       bool
	verifyJSON         bool
	verifyQuiet        bool
)

var repoVerifyCmd = &cobra.Command{
	Use:   "verify <publisher/name> <version>",
	Short: "Verify a single repository artifact's integrity",
	Long: `Verify a single repository artifact by calling the repository
VerifyArtifact RPC. Returns OK or one of BROKEN_MISSING_BLOB,
BROKEN_CHECKSUM_MISMATCH, BROKEN_MANIFEST_MISSING, BROKEN_LEDGER_MISSING,
QUARANTINED, REVOKED, or INCONCLUSIVE.

Exits non-zero on broken artifacts unless --quiet is set.

Examples:
  globular repository verify core@globular.io/echo 1.0.84
  globular repository verify core@globular.io/echo 1.0.84 --json
  globular repository verify core@globular.io/echo 1.0.84 --digest full`,
	Args: cobra.ExactArgs(2),
	RunE: runRepoVerify,
}

var (
	repairPlatform    string
	repairBuildNumber int64
	repairKind        string
	repairForce       bool
	repairAllowQuar   bool
	repairDryRun      bool
	repairYes         bool
	repairJSON        bool
)

var repoRepairCmd = &cobra.Command{
	Use:   "repair <publisher/name> <version>",
	Short: "Repair a broken repository artifact by re-importing from upstream",
	Long: `Repair a broken artifact by triggering RepairArtifact on the
repository service. The artifact's manifest must carry an UpstreamImport
record; the upstream source must still exist and be enabled.

Refuses REVOKED unconditionally. Refuses QUARANTINED unless
--allow-quarantine-override is set.

Examples:
  globular repository repair core@globular.io/echo 1.0.84
  globular repository repair core@globular.io/echo 1.0.84 --dry-run
  globular repository repair core@globular.io/echo 1.0.84 --force --json`,
	Args: cobra.ExactArgs(2),
	RunE: runRepoRepair,
}

var (
	explainPlatform    string
	explainBuildNumber int64
	explainKind        string
	explainJSON        bool
)

var repoExplainCmd = &cobra.Command{
	Use:   "explain <publisher/name> <version>",
	Short: "Explain an artifact's pipeline / publish / blob / ledger / installability state",
	Long: `Calls ExplainArtifact and renders the full operator-readable answer:
artifact_state, publish_state, blob presence + size, expected/actual digest,
ledger presence, manifest presence, installability, and recommended action.

Examples:
  globular repository explain core@globular.io/echo 1.0.84
  globular repository explain core@globular.io/echo 1.0.84 --json`,
	Args: cobra.ExactArgs(2),
	RunE: runRepoExplain,
}

func init() {
	defaultPlatform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)

	repoVerifyCmd.Flags().StringVar(&verifyPlatform, "platform", defaultPlatform, "Target platform (goos_goarch)")
	repoVerifyCmd.Flags().Int64Var(&verifyBuildNumber, "build-number", 0, "Specific build iteration (0 = latest PUBLISHED)")
	repoVerifyCmd.Flags().StringVar(&verifyBuildID, "build-id", "", "Specific build_id (optional)")
	repoVerifyCmd.Flags().StringVar(&verifyKind, "kind", "service", "Artifact kind: service|application|infrastructure|command|agent")
	repoVerifyCmd.Flags().StringVar(&verifyDigestMode, "digest", "stat", "Digest verification mode: full|stat|none")
	repoVerifyCmd.Flags().BoolVar(&verifySignature, "signature", false, "Verify signature (Phase CLI-B; currently a no-op stub)")
	repoVerifyCmd.Flags().BoolVar(&verifyLedger, "ledger", true, "Include ledger presence in result")
	repoVerifyCmd.Flags().BoolVar(&verifyJSON, "json", false, "Emit JSON output for automation / AI executor")
	repoVerifyCmd.Flags().BoolVar(&verifyQuiet, "quiet", false, "Always exit 0 even on broken artifacts")
	repoCmd.AddCommand(repoVerifyCmd)

	repoRepairCmd.Flags().StringVar(&repairPlatform, "platform", defaultPlatform, "Target platform (goos_goarch)")
	repoRepairCmd.Flags().Int64Var(&repairBuildNumber, "build-number", 0, "Specific build iteration (0 = latest)")
	repoRepairCmd.Flags().StringVar(&repairKind, "kind", "service", "Artifact kind")
	repoRepairCmd.Flags().BoolVar(&repairForce, "force", false, "Allow repair from any non-REVOKED state")
	repoRepairCmd.Flags().BoolVar(&repairAllowQuar, "allow-quarantine-override", false, "Permit repair of QUARANTINED rows (admin only)")
	repoRepairCmd.Flags().BoolVar(&repairDryRun, "dry-run", false, "Preview only — no state change")
	repoRepairCmd.Flags().BoolVar(&repairYes, "yes", false, "Skip confirmation prompt")
	repoRepairCmd.Flags().BoolVar(&repairJSON, "json", false, "Emit JSON output")
	repoCmd.AddCommand(repoRepairCmd)

	repoExplainCmd.Flags().StringVar(&explainPlatform, "platform", defaultPlatform, "Target platform")
	repoExplainCmd.Flags().Int64Var(&explainBuildNumber, "build-number", 0, "Specific build iteration (0 = latest)")
	repoExplainCmd.Flags().StringVar(&explainKind, "kind", "service", "Artifact kind")
	repoExplainCmd.Flags().BoolVar(&explainJSON, "json", false, "Emit JSON output")
	repoCmd.AddCommand(repoExplainCmd)
}

// ── Verify ─────────────────────────────────────────────────────────────────

func runRepoVerify(cmd *cobra.Command, args []string) error {
	publisher, name, err := parsePublisherName(args[0])
	if err != nil {
		return err
	}
	version := args[1]

	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ref := &repopb.ArtifactRef{
		PublisherId: publisher,
		Name:        name,
		Version:     version,
		Platform:    verifyPlatform,
		Kind:        resolveArtifactKind(verifyKind),
	}
	req := &repopb.VerifyArtifactRequest{
		Ref:             ref,
		BuildNumber:     verifyBuildNumber,
		BuildId:         verifyBuildID,
		VerifyDigest:    strings.EqualFold(verifyDigestMode, "full"),
		VerifySignature: verifySignature,
		IncludeLedger:   verifyLedger,
		IncludeManifest: true,
		IncludeBlob:     true,
	}
	resp, rpcErr := client.VerifyArtifact(req)
	if rpcErr != nil {
		return fmt.Errorf("verify: %w", rpcErr)
	}

	if verifyJSON {
		emitJSON(resp)
	} else {
		printVerifyTable([]*repopb.VerifyArtifactResponse{resp})
	}

	if !verifyQuiet && resp.GetStatus() != repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_OK {
		os.Exit(2)
	}
	return nil
}

func printVerifyTable(rows []*repopb.VerifyArtifactResponse) {
	fmt.Printf("%-32s %-12s %-14s %-26s %-12s %-11s %s\n",
		"ARTIFACT", "VERSION", "PLATFORM", "STATUS", "INSTALLABLE", "REPAIRABLE", "REASON")
	for _, r := range rows {
		ref := r.GetRef()
		artifact := fmt.Sprintf("%s/%s", ref.GetPublisherId(), ref.GetName())
		fmt.Printf("%-32s %-12s %-14s %-26s %-12s %-11s %s\n",
			truncStrDup(artifact, 32), truncStrDup(ref.GetVersion(), 12),
			truncStrDup(ref.GetPlatform(), 14),
			shortStatus(r.GetStatus()),
			yesNo(r.GetInstallable()), yesNo(r.GetRepairable()),
			r.GetReason())
	}
}

// ── Repair ─────────────────────────────────────────────────────────────────

func runRepoRepair(cmd *cobra.Command, args []string) error {
	publisher, name, err := parsePublisherName(args[0])
	if err != nil {
		return err
	}
	version := args[1]

	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ref := &repopb.ArtifactRef{
		PublisherId: publisher,
		Name:        name,
		Version:     version,
		Platform:    repairPlatform,
		Kind:        resolveArtifactKind(repairKind),
	}
	req := &repopb.RepairArtifactRequest{
		Ref:                     ref,
		BuildNumber:             repairBuildNumber,
		DryRun:                  repairDryRun,
		Force:                   repairForce,
		AllowQuarantineOverride: repairAllowQuar,
	}
	resp, rpcErr := client.RepairArtifact(req)
	if rpcErr != nil {
		return fmt.Errorf("repair: %w", rpcErr)
	}

	if repairJSON {
		emitJSON(resp)
	} else {
		fmt.Printf("artifact: %s\n", resp.GetArtifactKey())
		fmt.Printf("action:   %s\n", resp.GetAction())
		fmt.Printf("state:    %s → %s\n", resp.GetArtifactStateBefore(), resp.GetArtifactStateAfter())
		if resp.GetWorkflowRunId() != "" {
			fmt.Printf("run_id:   %s\n", resp.GetWorkflowRunId())
		}
		fmt.Printf("detail:   %s\n", resp.GetDetail())
	}

	switch resp.GetAction() {
	case "failed", "blocked_revoked", "blocked_quarantined":
		os.Exit(2)
	}
	return nil
}

// ── Explain ────────────────────────────────────────────────────────────────

func runRepoExplain(cmd *cobra.Command, args []string) error {
	publisher, name, err := parsePublisherName(args[0])
	if err != nil {
		return err
	}
	version := args[1]

	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ref := &repopb.ArtifactRef{
		PublisherId: publisher,
		Name:        name,
		Version:     version,
		Platform:    explainPlatform,
		Kind:        resolveArtifactKind(explainKind),
	}
	resp, rpcErr := client.ExplainArtifact(&repopb.ExplainArtifactRequest{
		Ref:         ref,
		BuildNumber: explainBuildNumber,
	})
	if rpcErr != nil {
		return fmt.Errorf("explain: %w", rpcErr)
	}

	if explainJSON {
		emitJSON(resp)
		if !resp.GetInstallable() {
			os.Exit(2)
		}
		return nil
	}

	fmt.Printf("artifact_key:   %s\n", resp.GetArtifactKey())
	fmt.Printf("artifact_state: %s\n", resp.GetArtifactState())
	fmt.Printf("publish_state:  %s\n", resp.GetPublishState())
	fmt.Printf("blob_key:       %s\n", resp.GetBlobKey())
	fmt.Printf("blob_present:   %v\n", resp.GetBlobPresent())
	if resp.GetExpectedSize() > 0 || resp.GetActualSize() > 0 {
		fmt.Printf("size:           expected=%d  actual=%d\n", resp.GetExpectedSize(), resp.GetActualSize())
	}
	if resp.GetExpectedDigest() != "" || resp.GetActualDigest() != "" {
		fmt.Printf("digest:         expected=%s\n", resp.GetExpectedDigest())
		if resp.GetActualDigest() != "" {
			fmt.Printf("                actual=%s\n", resp.GetActualDigest())
		}
	}
	fmt.Printf("manifest:       %s\n", presentMissing(resp.GetManifestPresent()))
	fmt.Printf("ledger:         %s\n", presentMissing(resp.GetLedgerPresent()))
	fmt.Printf("signature:      %s\n", resp.GetSignatureStatus())
	fmt.Printf("verify_status:  %s\n", shortStatus(resp.GetVerifyStatus()))
	fmt.Printf("installable:    %s\n", yesNo(resp.GetInstallable()))
	if avail := resp.GetSourceAvailability(); len(avail) > 0 {
		fmt.Printf("sources:\n")
		for _, entry := range avail {
			fmt.Printf("  %s\n", entry)
		}
	}
	if resp.GetRepairable() {
		fmt.Printf("repairable:     yes  (run: globular repository repair %s/%s %s)\n",
			resp.GetRef().GetPublisherId(), resp.GetRef().GetName(), resp.GetRef().GetVersion())
	}
	if resp.GetRecommendedAction() != "" {
		fmt.Printf("recommended:    %s\n", resp.GetRecommendedAction())
	}
	if resp.GetRelatedWorkflowRunId() != "" {
		fmt.Printf("workflow_run:   %s\n", resp.GetRelatedWorkflowRunId())
	}
	if resp.GetDetail() != "" {
		fmt.Printf("detail:         %s\n", resp.GetDetail())
	}

	if !resp.GetInstallable() {
		os.Exit(2)
	}
	return nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

func newRepoClient() (*repository_client.Repository_Service_Client, error) {
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr == "" {
		return nil, fmt.Errorf("cannot discover repository address")
	}
	c, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return nil, fmt.Errorf("connect to repository: %w", err)
	}
	if token := rootCfg.token; token != "" {
		c.SetToken(token)
	}
	return c, nil
}

func emitJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func shortStatus(s repopb.ArtifactVerifyStatus) string {
	const prefix = "ARTIFACT_VERIFY_"
	name := s.String()
	return strings.TrimPrefix(name, prefix)
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func presentMissing(v bool) string {
	if v {
		return "present"
	}
	return "missing"
}

func truncStrDup(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return s
	}
	return s[:n-1] + "…"
}

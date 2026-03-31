package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/template"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/identity"
	"github.com/globulario/services/golang/plan/versionutil"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ─── Flag variables ──────────────────────────────────────────────────────────

var (
	svcApplyService   string
	svcApplyVersion   string
	svcApplyPublisher string
	svcApplyRepoAddr  string
	svcApplyRepoInsec bool

	// Hidden flag to allow legacy imperative behaviour.
	svcDangerousImperative bool
)

// ─── Parent command ──────────────────────────────────────────────────────────

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Service installation commands",
}

// ─── services apply ──────────────────────────────────────────────────────────

var servicesApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "[REMOVED] Use 'globular services desired set' instead",
	Long: `REMOVED: Imperative install has been permanently removed.

All service installation must go through the declarative workflow:
  globular services desired set <service> <version>
  globular services apply-desired

This ensures signed plans, SHA256 verification, and automatic rollback.`,
	RunE: runServicesApply,
}

var servicesApplyDesiredCmd = &cobra.Command{
	Use:   "apply-desired",
	Short: "Install all services from the controller's desired state",
	Long: `Fetch the desired-state plan from the cluster controller and install
any services that are missing or at a different version locally.

Downloads run as the current user; privileged operations use sudo.

Example:
  globular services apply-desired --insecure`,
	RunE: runServicesApplyDesired,
}

var servicesSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Import installed services into the controller's desired state (idempotent)",
	Long: `Idempotent import: scans locally-installed services and creates matching
desired-state entries in the cluster controller. Safe to run multiple times —
existing entries are left unchanged.

After import, services transition from "Unmanaged" to "Installed" in the
4-layer state model (Artifact → Desired Release → Installed Observed → Runtime).

Example:
  globular services seed --insecure`,
	RunE: runServicesSeed,
}

// ─── desired subcommand group ────────────────────────────────────────────────

var servicesDesiredCmd = &cobra.Command{
	Use:   "desired",
	Short: "Manage the cluster's desired service state",
}

var servicesDesiredSetCmd = &cobra.Command{
	Use:   "set <service> <version>",
	Short: "Add or update a service in the desired state",
	Args:  cobra.ExactArgs(2),
	RunE:  runDesiredSet,
}

var servicesDesiredRemoveCmd = &cobra.Command{
	Use:   "remove <service>",
	Short: "Remove a service from the desired state",
	Args:  cobra.ExactArgs(1),
	RunE:  runDesiredRemove,
}

var servicesDesiredListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services in the desired state",
	RunE:  runDesiredList,
}

var servicesDesiredDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare desired state with locally-installed versions",
	RunE:  runDesiredDiff,
}

// ─── list-desired ────────────────────────────────────────────────────────────

var servicesListDesiredCmd = &cobra.Command{
	Use:   "list-desired",
	Short: "List desired vs installed services and compare state hashes",
	Long: `Fetch the desired state from the cluster controller and compare it
with locally-installed service versions. Computes and displays the
desired and applied state hashes so you can diagnose hash mismatches.

Example:
  globular services list-desired --insecure`,
	RunE: runServicesListDesired,
}

// ─── adopt-installed ─────────────────────────────────────────────────────────

var servicesAdoptInstalledCmd = &cobra.Command{
	Use:   "adopt-installed",
	Short: "Import locally-installed services into the desired state (alias for seed)",
	Long: `Alias for 'services seed'. Idempotent import of all locally-installed
services into the controller's desired state so they become declaratively
managed and visible across all 4 state layers.

Example:
  globular services adopt-installed --insecure`,
	RunE: runServicesSeed,
}

var servicesRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Diagnose and repair state alignment across all layers",
	Long: `Cross-references the 4 state layers (artifact/repository, desired release,
installed observed, runtime health) and reports per-package alignment status.

Without --dry-run, also repairs missing desired-state entries by importing
from installed packages.

Example:
  globular services repair --insecure
  globular services repair --dry-run --insecure`,
	RunE: runServicesRepair,
}

var servicesRepairDryRun bool

func init() {
	servicesApplyCmd.Flags().StringVar(&svcApplyService, "service", "", "Service name (required)")
	servicesApplyCmd.Flags().StringVar(&svcApplyVersion, "version", "", "Service version (required)")
	servicesApplyCmd.Flags().StringVar(&svcApplyPublisher, "publisher", "core@globular.io", "Publisher ID")
	servicesApplyCmd.Flags().StringVar(&svcApplyRepoAddr, "repository", "", "Repository gRPC endpoint (auto-discovered if empty)")
	servicesApplyCmd.Flags().BoolVar(&svcApplyRepoInsec, "repository-insecure", false, "Use plaintext for repository connection")
	// --dangerous-imperative flag removed: imperative install is permanently disabled.
	// The flag variable is kept to avoid breaking CLI flag parsing for scripts
	// that might still pass it, but it has no effect.
	servicesApplyCmd.Flags().BoolVar(&svcDangerousImperative, "dangerous-imperative", false, "Removed: imperative install is permanently disabled")
	_ = servicesApplyCmd.Flags().MarkHidden("dangerous-imperative")

	servicesApplyDesiredCmd.Flags().StringVar(&svcApplyPublisher, "publisher", "core@globular.io", "Publisher ID")
	servicesApplyDesiredCmd.Flags().StringVar(&svcApplyRepoAddr, "repository", "", "Repository gRPC endpoint (auto-discovered if empty)")
	servicesApplyDesiredCmd.Flags().BoolVar(&svcApplyRepoInsec, "repository-insecure", false, "Use plaintext for repository connection")

	servicesDesiredCmd.AddCommand(servicesDesiredSetCmd)
	servicesDesiredCmd.AddCommand(servicesDesiredRemoveCmd)
	servicesDesiredCmd.AddCommand(servicesDesiredListCmd)
	servicesDesiredCmd.AddCommand(servicesDesiredDiffCmd)

	servicesRepairCmd.Flags().BoolVar(&servicesRepairDryRun, "dry-run", false, "Report only — do not repair")

	servicesCmd.AddCommand(servicesApplyCmd)
	servicesCmd.AddCommand(servicesApplyDesiredCmd)
	servicesCmd.AddCommand(servicesSeedCmd)
	servicesCmd.AddCommand(servicesDesiredCmd)
	servicesCmd.AddCommand(servicesAdoptInstalledCmd)
	servicesCmd.AddCommand(servicesListDesiredCmd)
	servicesCmd.AddCommand(servicesRepairCmd)
}

// ─── apply ───────────────────────────────────────────────────────────────────

func runServicesApply(cmd *cobra.Command, args []string) error {
	// Imperative install has been permanently removed. All service installation
	// must go through the declarative desired-state workflow, which uses signed
	// plans, SHA256 verification, and automatic rollback.
	return fmt.Errorf("imperative install has been removed\n\n" +
		"Use the declarative workflow instead:\n" +
		"  globular services desired set <service> <version>\n" +
		"  globular services apply-desired\n\n" +
		"To import existing installations into the desired state:\n" +
		"  globular services seed\n\n" +
		"To diagnose and repair alignment across all 4 state layers:\n" +
		"  globular services repair")
}

// ─── apply-desired ───────────────────────────────────────────────────────────

func runServicesApplyDesired(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	resolveRepositoryAddr(cmd)

	desired, err := fetchDesiredState()
	if err != nil {
		return fmt.Errorf("fetch desired state (controller=%s, insecure=%v): %w\n"+
			"Hint: try globular services apply-desired --controller <addr>:12000 --insecure", rootCfg.controllerAddr, rootCfg.insecure, err)
	}

	if len(desired) == 0 {
		fmt.Println("No desired services configured — auto-seeding from installed services…")
		seeded, err := seedDesiredState()
		if err != nil || len(seeded) == 0 {
			// Controller seed failed (e.g. no node reports yet) — fall back to
			// scanning locally installed globular-*.service systemd units.
			if err != nil {
				fmt.Printf("  Controller seed unavailable: %v\n", err)
			}
			fmt.Println("  Falling back to local systemd scan…")
			seeded, err = seedFromLocalUnits()
			if err != nil {
				return fmt.Errorf("local seed failed: %w", err)
			}
		}
		if len(seeded) == 0 {
			fmt.Println("No installed services found. Nothing to apply.")
			return nil
		}
		fmt.Printf("Seeded %d service(s) into desired state:\n", len(seeded))
		for _, ds := range seeded {
			fmt.Printf("  • %s@%s\n", ds.GetServiceId(), ds.GetVersion())
		}
		desired = seeded
	}

	var installed, skipped, failed int
	for _, ds := range desired {
		name := ds.GetServiceId()
		version := ds.GetVersion()
		if cv, err := versionutil.Canonical(version); err == nil {
			version = cv
		}

		localVer := readLocalVersion(name)
		if localVer == version {
			fmt.Printf("  ✓ %s@%s — already installed\n", name, version)
			skipped++
			continue
		}

		fmt.Printf("→ Installing %s@%s (local: %s) …\n", name, version, orDash(localVer))

		tgzPath, err := downloadServiceArtifact(name, version, svcApplyPublisher)
		if err != nil {
			fmt.Printf("  ✕ %s@%s — download failed: %v\n", name, version, err)
			failed++
			continue
		}

		unitName, err := installServiceTgz(tgzPath, name)
		os.Remove(tgzPath)
		if err != nil {
			fmt.Printf("  ✕ %s@%s — install failed: %v\n", name, version, err)
			failed++
			continue
		}

		if unitName != "" {
			if err := systemctlActivate(unitName); err != nil {
				fmt.Printf("  ✕ %s@%s — systemctl failed: %v\n", name, version, err)
				failed++
				continue
			}
		}

		if err := writeVersionMarker(name, version); err != nil {
			fmt.Printf("  ✕ %s@%s — version marker failed: %v\n", name, version, err)
			failed++
			continue
		}

		fmt.Printf("  ✓ %s@%s installed\n", name, version)
		installed++
	}

	fmt.Printf("\nSummary: %d installed, %d skipped (up-to-date), %d failed\n",
		installed, skipped, failed)
	if failed > 0 {
		return fmt.Errorf("%d service(s) failed to install", failed)
	}
	return nil
}

// ─── seed ───────────────────────────────────────────────────────────────────

// seedDesiredState calls the SeedDesiredState RPC to import installed services
// into the controller's desired state. Returns the list of seeded services.
func seedDesiredState() ([]*cluster_controllerpb.DesiredService, error) {
	conn, err := controllerClient()
	if err != nil {
		return nil, fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	resp, err := cc.SeedDesiredState(ctx, &cluster_controllerpb.SeedDesiredStateRequest{
		Mode: cluster_controllerpb.SeedDesiredStateRequest_IMPORT_FROM_INSTALLED,
	})
	if err != nil {
		return nil, fmt.Errorf("SeedDesiredState: %w", err)
	}
	return resp.GetServices(), nil
}

// seedFromLocalUnits scans locally installed globular-*.service systemd units,
// registers each one in the controller's desired state via UpsertDesiredService,
// and returns the resulting desired services list.
func seedFromLocalUnits() ([]*cluster_controllerpb.DesiredService, error) {
	// List active globular-*.service units
	out, err := exec.Command("systemctl", "list-units", "globular-*.service",
		"--no-legend", "--no-pager", "--plain").Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl list-units: %w", err)
	}

	// Infrastructure units not managed by the desired-services model.
	// Control-plane services (node-agent, cluster-controller, cluster-doctor)
	// ARE managed — they participate in desired state and reconciliation.
	skip := map[string]bool{
		"envoy": true, "etcd": true, "minio": true,
		"gateway": true, "xds": true,
	}

	conn, err := controllerClient()
	if err != nil {
		return nil, fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	var lastResp *cluster_controllerpb.DesiredState

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0] // e.g. "globular-authentication.service"
		// Canonicalize via shared identity helper: strips prefix/suffix,
		// normalizes underscores → hyphens, resolves through registry.
		base := identity.UnitToServiceID(unit)
		if base == "" || skip[base] {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
		resp, err := cc.UpsertDesiredService(ctx, &cluster_controllerpb.UpsertDesiredServiceRequest{
			Service: &cluster_controllerpb.DesiredService{
				ServiceId: base,
				Version:   "0.0.1",
			},
		})
		cancel()
		if err != nil {
			fmt.Printf("  warning: could not register %s: %v\n", base, err)
			continue
		}
		lastResp = resp
	}

	if lastResp == nil {
		return nil, nil
	}
	return lastResp.GetServices(), nil
}

func runServicesSeed(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	services, err := seedDesiredState()
	if err != nil {
		return fmt.Errorf("%w\nHint: try --controller <addr>:12000 --insecure", err)
	}

	if len(services) == 0 {
		fmt.Println("No services seeded (no installed services reported by nodes).")
		return nil
	}

	fmt.Printf("Desired state seeded with %d service(s):\n", len(services))
	for _, ds := range services {
		fmt.Printf("  • %s@%s\n", ds.GetServiceId(), ds.GetVersion())
	}
	return nil
}

// ─── repair ──────────────────────────────────────────────────────────────────

func runServicesRepair(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	client := cluster_controllerpb.NewResourcesServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	report, err := client.RepairStateAlignment(ctx, &cluster_controllerpb.RepairStateAlignmentRequest{
		DryRun: servicesRepairDryRun,
	})
	if err != nil {
		return fmt.Errorf("RepairStateAlignment: %w", err)
	}

	if servicesRepairDryRun {
		fmt.Println("DRY RUN — no changes applied")
	}
	if report.RepositoryAddr != "" {
		fmt.Printf("Repository: %s\n", report.RepositoryAddr)
	}
	fmt.Println()

	// Print per-package table.
	fmt.Printf("%-35s %-16s %-12s %-12s %-12s %s\n",
		"PACKAGE", "KIND", "INSTALLED", "DESIRED", "REPO", "STATUS")
	fmt.Printf("%-35s %-16s %-12s %-12s %-12s %s\n",
		strings.Repeat("─", 35), strings.Repeat("─", 16),
		strings.Repeat("─", 12), strings.Repeat("─", 12),
		strings.Repeat("─", 12), strings.Repeat("─", 20))

	for _, pkg := range report.Packages {
		fmt.Printf("%-35s %-16s %-12s %-12s %-12s %s\n",
			pkg.Name, pkg.Kind,
			orDash(fmtVer(pkg.InstalledVersion, pkg.InstalledBuildNum)),
			orDash(fmtVer(pkg.DesiredVersion, pkg.DesiredBuildNum)),
			orDash(fmtVer(pkg.RepoVersion, pkg.RepoBuildNum)),
			pkg.Status)
	}

	// Summary.
	fmt.Println()
	fmt.Printf("Aligned: %d  Repaired: %d  Drifted: %d  Unmanaged: %d  Missing in repo: %d\n",
		report.Aligned, report.Repaired, report.Drifted, report.Unmanaged, report.MissingInRepo)

	if report.Drifted > 0 {
		fmt.Println("\nDrifted packages have installed versions different from desired.")
		fmt.Println("The controller reconciler will converge them automatically.")
	}
	if report.Unmanaged > 0 {
		fmt.Println("\nUnmanaged packages are installed but have no desired release.")
		fmt.Printf("Run without --dry-run to import them: globular services repair\n")
	}
	return nil
}

// ─── desired set ─────────────────────────────────────────────────────────────

func runDesiredSet(cmd *cobra.Command, args []string) error {
	serviceID := args[0]
	version := args[1]
	if cv, err := versionutil.Canonical(version); err == nil {
		version = cv
	}

	autoDiscoverController(cmd)

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	resp, err := cc.UpsertDesiredService(ctx, &cluster_controllerpb.UpsertDesiredServiceRequest{
		Service: &cluster_controllerpb.DesiredService{
			ServiceId: serviceID,
			Version:   version,
		},
	})
	if err != nil {
		return fmt.Errorf("UpsertDesiredService: %w", err)
	}

	fmt.Printf("OK: desired state updated (revision %s)\n", resp.GetRevision())
	for _, ds := range resp.GetServices() {
		fmt.Printf("  • %s@%s\n", ds.GetServiceId(), ds.GetVersion())
	}
	return nil
}

// ─── desired remove ──────────────────────────────────────────────────────────

func runDesiredRemove(cmd *cobra.Command, args []string) error {
	serviceID := args[0]

	autoDiscoverController(cmd)

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	resp, err := cc.RemoveDesiredService(ctx, &cluster_controllerpb.RemoveDesiredServiceRequest{
		ServiceId: serviceID,
	})
	if err != nil {
		return fmt.Errorf("RemoveDesiredService: %w", err)
	}

	fmt.Printf("OK: %s removed from desired state (revision %s)\n", serviceID, resp.GetRevision())
	if len(resp.GetServices()) == 0 {
		fmt.Println("Desired state is now empty.")
	} else {
		for _, ds := range resp.GetServices() {
			fmt.Printf("  • %s@%s\n", ds.GetServiceId(), ds.GetVersion())
		}
	}
	return nil
}

// ─── desired list ────────────────────────────────────────────────────────────

func runDesiredList(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	desired, err := fetchDesiredState()
	if err != nil {
		return fmt.Errorf("fetch desired state: %w", err)
	}

	if len(desired) == 0 {
		fmt.Println("No desired services configured.")
		return nil
	}

	fmt.Printf("%-40s %s\n", "SERVICE", "VERSION")
	fmt.Printf("%-40s %s\n", strings.Repeat("─", 40), strings.Repeat("─", 20))
	for _, ds := range desired {
		fmt.Printf("%-40s %s\n", ds.GetServiceId(), fmtVer(ds.GetVersion(), ds.GetBuildNumber()))
	}
	return nil
}

// ─── desired diff ────────────────────────────────────────────────────────────

func runDesiredDiff(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	desired, err := fetchDesiredState()
	if err != nil {
		return fmt.Errorf("fetch desired state: %w", err)
	}

	if len(desired) == 0 {
		fmt.Println("No desired services configured.")
		return nil
	}

	fmt.Printf("%-40s %-20s %-20s %s\n", "SERVICE", "DESIRED", "LOCAL", "STATUS")
	fmt.Printf("%-40s %-20s %-20s %s\n",
		strings.Repeat("─", 40), strings.Repeat("─", 20), strings.Repeat("─", 20), strings.Repeat("─", 12))

	var drifted int
	for _, ds := range desired {
		name := ds.GetServiceId()
		desiredVer := ds.GetVersion()
		localVer := readLocalVersion(name)

		status := "✓ current"
		if localVer == "" {
			status = "✕ missing"
			drifted++
		} else if localVer != desiredVer {
			status = "~ drift"
			drifted++
		}
		fmt.Printf("%-40s %-20s %-20s %s\n", name, desiredVer, orDash(localVer), status)
	}

	if drifted > 0 {
		fmt.Printf("\n%d service(s) need attention. Run: globular services apply-desired\n", drifted)
	} else {
		fmt.Println("\nAll services at desired versions.")
	}
	return nil
}

// ─── Repository discovery ────────────────────────────────────────────────────

// resolveRepositoryAddr auto-discovers the repository gRPC endpoint when
// --repository was not explicitly provided. Uses config.ResolveServiceAddr
// which queries etcd for service registrations.
func resolveRepositoryAddr(cmd *cobra.Command) {
	if cmd.Flags().Changed("repository") {
		return // user set it explicitly
	}
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr != "" {
		svcApplyRepoAddr = addr
		fmt.Printf("Auto-discovered repository: %s\n", svcApplyRepoAddr)
	}
	if svcApplyRepoAddr == "" {
		svcApplyRepoAddr = "localhost:10007" // common default
	}
}

// ─── Artifact download ───────────────────────────────────────────────────────

func downloadServiceArtifact(service, version, publisher string) (string, error) {
	platform := runtime.GOOS + "_" + runtime.GOARCH
	ref := &repositorypb.ArtifactRef{
		PublisherId: publisher,
		Name:        service,
		Version:     version,
		Platform:    platform,
		Kind:        repositorypb.ArtifactKind_SERVICE,
	}

	conn, err := dialRepository()
	if err != nil {
		return "", fmt.Errorf("dial repository %s: %w", svcApplyRepoAddr, err)
	}
	defer conn.Close()

	client := repositorypb.NewPackageRepositoryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stream, err := client.DownloadArtifact(ctx, &repositorypb.DownloadArtifactRequest{Ref: ref})
	if err != nil {
		return "", fmt.Errorf("DownloadArtifact %s/%s@%s: %w", publisher, service, version, err)
	}

	tmp, err := os.CreateTemp("", "globular-svc-*.tgz")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return "", fmt.Errorf("receive chunk: %w", err)
		}
		if _, err := tmp.Write(resp.GetData()); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return "", fmt.Errorf("write chunk: %w", err)
		}
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func dialRepository() (*grpc.ClientConn, error) {
	if svcApplyRepoInsec {
		// TLS with skip-verify — all services require TLS.
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))}
		if rootCfg.token != "" {
			opts = append(opts, grpc.WithPerRPCCredentials(tokenCredentials{token: rootCfg.token}))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return grpc.DialContext(ctx, svcApplyRepoAddr, opts...)
	}
	// TLS — always use TLS for the repository, regardless of rootCfg.insecure
	// (which may be set for the controller's plain gRPC connection).
	creds, err := getTLSCredentials()
	if err != nil {
		return nil, fmt.Errorf("repository TLS: %w", err)
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	if rootCfg.token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(tokenCredentials{token: rootCfg.token}))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return grpc.DialContext(ctx, svcApplyRepoAddr, opts...)
}

// ─── Tgz extraction & install ────────────────────────────────────────────────

const (
	defaultBinDir     = "/usr/lib/globular/bin"
	defaultSystemdDir = "/etc/systemd/system"
	defaultConfigDir  = "/etc/globular"
)

// installServiceTgz extracts the tgz to a temp staging directory (as the
// current user) then uses sudo to copy files to privileged locations.
// This allows the command to run as a regular user who has TLS certificates,
// escalating only for file installs and systemctl.
func installServiceTgz(tgzPath, service string) (unitName string, err error) {
	binDir := envOr("GLOBULAR_INSTALL_BIN_DIR", defaultBinDir)
	systemdDir := envOr("GLOBULAR_INSTALL_SYSTEMD_DIR", defaultSystemdDir)
	configDir := envOr("GLOBULAR_INSTALL_CONFIG_DIR", defaultConfigDir)
	stateDir := envOr("GLOBULAR_STATE_DIR", "/var/lib/globular")
	prefix := filepath.Dir(binDir) // e.g. /usr/lib/globular

	// Template variables for systemd unit files shipped as Go templates.
	unitVars := struct {
		Prefix   string
		StateDir string
	}{
		Prefix:   prefix,
		StateDir: stateDir,
	}

	// ── Stage 1: extract tgz to a temp staging dir (no sudo needed) ─────

	staging, err := os.MkdirTemp("", "globular-stage-*")
	if err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	f, err := os.Open(tgzPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	// stagedFile tracks a file extracted to staging and its final destination.
	type stagedFile struct {
		stagePath string
		destPath  string
		mode      os.FileMode
	}
	var binFiles, unitFiles, cfgFiles []stagedFile

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}

		name := strings.TrimLeft(hdr.Name, "./")
		isUnit := false
		var dest string
		var bucket *[]stagedFile

		switch {
		case strings.HasPrefix(name, "bin/"):
			dest = filepath.Join(binDir, filepath.Base(name))
			bucket = &binFiles
		case strings.HasPrefix(name, "systemd/"), strings.HasPrefix(name, "units/"):
			dest = filepath.Join(systemdDir, filepath.Base(name))
			unitName = filepath.Base(name)
			isUnit = true
			bucket = &unitFiles
		case strings.HasPrefix(name, "config/"):
			dest = filepath.Join(configDir, service, strings.TrimPrefix(name, "config/"))
			bucket = &cfgFiles
		default:
			continue
		}

		// Read entry into memory for template rendering.
		var content bytes.Buffer
		if _, err := io.Copy(&content, tr); err != nil {
			return "", fmt.Errorf("read %s: %w", name, err)
		}

		writeData := content.Bytes()
		if isUnit && bytes.Contains(writeData, []byte("{{")) {
			tmpl, err := template.New(name).Parse(content.String())
			if err != nil {
				return "", fmt.Errorf("parse unit template %s: %w", name, err)
			}
			var rendered bytes.Buffer
			if err := tmpl.Execute(&rendered, unitVars); err != nil {
				return "", fmt.Errorf("render unit template %s: %w", name, err)
			}
			writeData = rendered.Bytes()
		}

		// Write to staging dir.
		stageFile := filepath.Join(staging, name)
		if err := os.MkdirAll(filepath.Dir(stageFile), 0o755); err != nil {
			return "", fmt.Errorf("staging mkdir: %w", err)
		}
		if err := os.WriteFile(stageFile, writeData, hdr.FileInfo().Mode()); err != nil {
			return "", fmt.Errorf("staging write %s: %w", name, err)
		}

		*bucket = append(*bucket, stagedFile{
			stagePath: stageFile,
			destPath:  dest,
			mode:      hdr.FileInfo().Mode(),
		})
	}

	// ── Stage 2: install files using sudo ───────────────────────────────

	isRoot := os.Geteuid() == 0

	// Ensure service working directory exists.
	wdDir := filepath.Join(stateDir, service)
	if err := sudoMkdirAll(wdDir, isRoot); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", wdDir, err)
	}

	allFiles := make([]stagedFile, 0, len(binFiles)+len(unitFiles)+len(cfgFiles))
	allFiles = append(allFiles, binFiles...)
	allFiles = append(allFiles, unitFiles...)
	allFiles = append(allFiles, cfgFiles...)

	for _, sf := range allFiles {
		if err := sudoInstallFile(sf.stagePath, sf.destPath, sf.mode, isRoot); err != nil {
			return "", fmt.Errorf("install %s: %w", sf.destPath, err)
		}
	}

	// daemon-reload after writing unit files
	if len(unitFiles) > 0 {
		if err := sudoSystemctl(isRoot, "daemon-reload"); err != nil {
			return "", err
		}
	}

	return unitName, nil
}

// ─── sudo helpers ────────────────────────────────────────────────────────────

// sudoRun executes a command, prepending "sudo" if the process is not root.
func sudoRun(isRoot bool, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if isRoot {
		cmd = exec.CommandContext(ctx, name, args...)
	} else {
		cmdArgs := append([]string{name}, args...)
		cmd = exec.CommandContext(ctx, "sudo", cmdArgs...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// sudoMkdirAll creates a directory tree, using sudo if needed.
func sudoMkdirAll(dir string, isRoot bool) error {
	return sudoRun(isRoot, "mkdir", "-p", dir)
}

// sudoInstallFile copies a staged file to its final destination using
// "install" (preserves mode), with sudo if the process is not root.
func sudoInstallFile(src, dest string, mode os.FileMode, isRoot bool) error {
	// Ensure parent directory exists.
	if err := sudoMkdirAll(filepath.Dir(dest), isRoot); err != nil {
		return err
	}
	modeStr := fmt.Sprintf("%04o", mode)
	return sudoRun(isRoot, "install", "-m", modeStr, src, dest)
}

// ─── systemctl helpers ───────────────────────────────────────────────────────

// sudoSystemctl runs systemctl with sudo if not already root.
func sudoSystemctl(isRoot bool, args ...string) error {
	return sudoRun(isRoot, "systemctl", args...)
}

func systemctlActivate(unit string) error {
	isRoot := os.Geteuid() == 0
	for _, action := range []string{"enable", "restart"} {
		if err := sudoSystemctl(isRoot, action, unit); err != nil {
			return fmt.Errorf("systemctl %s %s: %w", action, unit, err)
		}
	}
	return nil
}


// ─── Version marker ──────────────────────────────────────────────────────────

func writeVersionMarker(service, version string) error {
	path := versionutil.MarkerPath(service)
	isRoot := os.Geteuid() == 0

	if err := sudoMkdirAll(filepath.Dir(path), isRoot); err != nil {
		return err
	}

	// Write to a temp file (as current user) then sudo-install it.
	tmp, err := os.CreateTemp("", "globular-ver-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(version); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	return sudoInstallFile(tmpPath, path, 0o644, isRoot)
}

func readLocalVersion(service string) string {
	path := versionutil.MarkerPath(service)
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// ─── Controller desired state ────────────────────────────────────────────────

func fetchDesiredState() ([]*cluster_controllerpb.DesiredService, error) {
	conn, err := controllerClient()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	resp, err := cc.GetDesiredState(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.GetServices(), nil
}

// ─── Node ID from state file ─────────────────────────────────────────────────

type nodeState struct {
	NodeID             string `json:"node_id"`
	ControllerEndpoint string `json:"controller_endpoint"`
	ControllerInsecure bool   `json:"controller_insecure"`
}

func readNodeState() (*nodeState, error) {
	stateDir := envOr("GLOBULAR_STATE_DIR", "/var/lib/globular")
	path := filepath.Join(stateDir, "nodeagent", "state.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s nodeState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &s, nil
}

// autoDiscoverController sets rootCfg.controllerAddr (and rootCfg.insecure)
// from the node-agent state file when --controller was not explicitly provided.
//
// Discovery sources (in priority order):
//  1. node-agent state.json (controller_endpoint + controller_insecure)
//  2. NODE_AGENT_CONTROLLER_ENDPOINT environment variable
//  3. Default controller port on localhost ("localhost:12000")
//
// The insecure flag is inferred from:
//  1. --insecure flag explicitly set by the user (never overridden)
//  2. controller_insecure field in state.json
//  3. NODE_AGENT_INSECURE=true environment variable
func autoDiscoverController(cmd *cobra.Command) {
	if cmd.Flags().Changed("controller") {
		return
	}

	var discovered string
	var source string
	isInsecure := false

	// Priority 1: node-agent state file
	if ns, err := readNodeState(); err == nil && ns.ControllerEndpoint != "" {
		ep := ns.ControllerEndpoint
		// Reject unroutable bind addresses — 0.0.0.0 is not a valid client endpoint.
		if !strings.HasPrefix(ep, "0.0.0.0:") {
			discovered = ep
			isInsecure = ns.ControllerInsecure
			source = "node-agent state"
		}
	}

	// Priority 2: environment variable (same as node-agent uses)
	if discovered == "" {
		if ep := strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_ENDPOINT")); ep != "" {
			discovered = ep
			source = "NODE_AGENT_CONTROLLER_ENDPOINT"
		}
	}

	// Priority 3: cluster controller is running locally on the well-known port
	if discovered == "" {
		discovered = "localhost:12000"
		source = "default controller port"
	}

	// Infer insecure from env or node-agent systemd drop-in
	if !isInsecure {
		if strings.EqualFold(os.Getenv("NODE_AGENT_INSECURE"), "true") {
			isInsecure = true
		} else if isNodeAgentInsecureFromSystemd() {
			isInsecure = true
		}
	}

	rootCfg.controllerAddr = discovered
	if isInsecure && !cmd.Flags().Changed("insecure") {
		rootCfg.insecure = true
	}
	fmt.Printf("Auto-discovered controller: %s (%s)\n", rootCfg.controllerAddr, source)
}

// isNodeAgentInsecureFromSystemd checks the node-agent systemd drop-in for
// NODE_AGENT_INSECURE=true. This covers the case where the CLI user's shell
// doesn't have the env var set but the node-agent service does.
func isNodeAgentInsecureFromSystemd() bool {
	paths := []string{
		"/etc/systemd/system/globular-node-agent.service.d/insecure.conf",
		"/etc/systemd/system/globular-node-agent.service.d/override.conf",
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "NODE_AGENT_INSECURE=true") ||
			strings.Contains(string(b), `NODE_AGENT_INSECURE="true"`) {
			return true
		}
	}
	return false
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// fmtVer formats a version string with an optional build number suffix.
func fmtVer(version string, buildNumber int64) string {
	if version == "" {
		return ""
	}
	if buildNumber > 0 {
		return fmt.Sprintf("%s+b%d", version, buildNumber)
	}
	return version
}

// ─── list-desired implementation ─────────────────────────────────────────────

func runServicesListDesired(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	// 1. Fetch desired state from controller.
	desired, err := fetchDesiredState()
	if err != nil {
		return fmt.Errorf("fetch desired state (controller=%s, insecure=%v): %w\n"+
			"Hint: try globular services list-desired --controller <addr>:12000 --insecure",
			rootCfg.controllerAddr, rootCfg.insecure, err)
	}

	// 2. Build desired service map (canonical name → version), same as
	//    controller's stableServiceDesiredHash.
	desiredMap := make(map[string]string, len(desired))
	for _, ds := range desired {
		key, _ := identity.NormalizeServiceKey(ds.GetServiceId())
		if key == "" {
			continue
		}
		ver := ds.GetVersion()
		if cv, err := versionutil.Canonical(ver); err == nil {
			ver = cv
		}
		desiredMap[key] = ver
	}
	desiredHash := stableHash(desiredMap)

	// 3. Scan locally-installed services (version markers + systemd), same
	//    sources the node agent uses for computeAppliedServicesHash.
	installedMap := scanLocalInstalledServices()
	installedHash := stableHash(installedMap)

	// 4. Merge keys from both maps to show a unified diff.
	allKeys := make(map[string]bool)
	for k := range desiredMap {
		allKeys[k] = true
	}
	for k := range installedMap {
		allKeys[k] = true
	}
	sorted := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	// 5. Print comparison table.
	fmt.Printf("%-35s %-18s %-18s %s\n", "SERVICE", "DESIRED", "INSTALLED", "STATUS")
	fmt.Printf("%-35s %-18s %-18s %s\n",
		strings.Repeat("─", 35), strings.Repeat("─", 18),
		strings.Repeat("─", 18), strings.Repeat("─", 14))

	var drifted, extra, missing int
	for _, svc := range sorted {
		dVer := desiredMap[svc]
		iVer := installedMap[svc]
		status := "✓ match"
		switch {
		case dVer == "" && iVer != "":
			status = "⊕ extra (not desired)"
			extra++
		case dVer != "" && iVer == "":
			status = "✕ missing"
			missing++
		case dVer != iVer:
			status = "~ version drift"
			drifted++
		}
		fmt.Printf("%-35s %-18s %-18s %s\n", svc, orDash(dVer), orDash(iVer), status)
	}

	// 6. Print hash comparison.
	fmt.Println()
	fmt.Printf("Desired hash:   %s\n", desiredHash)
	fmt.Printf("Installed hash: %s\n", installedHash)
	if desiredHash == installedHash {
		fmt.Println("Hashes MATCH ✓")
	} else {
		fmt.Println("Hashes DIFFER ✕")
	}

	// 7. Summary.
	fmt.Println()
	if extra+missing+drifted > 0 {
		fmt.Printf("Summary: %d missing, %d version drift, %d extra (not in desired state)\n",
			missing, drifted, extra)
		if extra > 0 {
			fmt.Println("\nExtra services contribute to the installed hash but not the desired hash.")
			fmt.Println("To fix: either add them to desired state or remove stale version markers:")
			fmt.Println("  sudo rm -rf /var/lib/globular/version-markers/<service>")
			fmt.Println("  sudo systemctl restart globular-node-agent.service")
		}
		if missing > 0 {
			fmt.Println("\nMissing services are in desired state but not installed locally.")
			fmt.Println("To fix: globular services apply-desired --insecure")
		}
	} else if len(sorted) == 0 {
		fmt.Println("No desired services configured and no installed services found.")
	} else {
		fmt.Println("All services match. If the controller still reports a hash mismatch,")
		fmt.Println("restart the node agent to force a fresh status report:")
		fmt.Println("  sudo systemctl restart globular-node-agent.service")
	}
	return nil
}

// stableHash computes a state hash identical to the controller's
// stableServiceDesiredHash / node agent's computeAppliedServicesHash.
// Format: sorted "key=version;" entries → SHA256 → "services:<hex>".
func stableHash(versions map[string]string) string {
	if len(versions) == 0 {
		return "services:none"
	}
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(versions[k])
		b.WriteString(";")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "services:" + hex.EncodeToString(sum[:])
}

// scanLocalInstalledServices discovers services the same way the node agent
// does: version markers first, then active systemd units as fallback.
// Returns canonical-name → version map.
func scanLocalInstalledServices() map[string]string {
	installed := make(map[string]string)

	// Source 1: version markers.
	markerRoot := versionutil.BaseDir()
	entries, err := os.ReadDir(markerRoot)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			verPath := filepath.Join(markerRoot, e.Name(), "version")
			data, err := os.ReadFile(verPath)
			if err != nil {
				continue
			}
			ver := strings.TrimSpace(string(data))
			if ver == "" {
				continue
			}
			if cv, err := versionutil.Canonical(ver); err == nil {
				ver = cv
			}
			key, _ := identity.NormalizeServiceKey(e.Name())
			if key != "" {
				installed[key] = ver
			}
		}
	}

	// Source 2: service config JSON files (same as node agent loadServiceConfigs).
	cfgRoot := config.GetServicesConfigDir()
	cfgEntries, err := os.ReadDir(cfgRoot)
	if err == nil {
		for _, e := range cfgEntries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(cfgRoot, e.Name()))
			if err != nil {
				continue
			}
			var raw map[string]interface{}
			if json.Unmarshal(data, &raw) != nil {
				continue
			}
			svc := ""
			for _, field := range []string{"Name", "ServiceName", "service_name", "service"} {
				if v, ok := raw[field].(string); ok && v != "" {
					svc = v
					break
				}
			}
			key, _ := identity.NormalizeServiceKey(svc)
			if key == "" {
				continue
			}
			if _, exists := installed[key]; exists {
				continue // marker already found
			}
			ver := ""
			for _, field := range []string{"Version", "version"} {
				if v, ok := raw[field].(string); ok && v != "" {
					ver = v
					break
				}
			}
			if ver == "" {
				continue
			}
			if cv, err := versionutil.Canonical(ver); err == nil {
				ver = cv
			}
			installed[key] = ver
		}
	}

	// Source 3: active systemd units (same as node agent loadSystemdUnits).
	// Infrastructure services are excluded, matching the node agent's infra set.
	infra := map[string]bool{
		"etcd": true, "minio": true, "envoy": true,
		"xds": true, "gateway": true,
	}
	out, err := exec.Command("systemctl", "list-units",
		"--type=service", "--state=active", "--no-legend", "--no-pager",
		"globular-*.service").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			key, _ := identity.NormalizeServiceKey(fields[0])
			if key == "" || infra[key] {
				continue
			}
			if _, exists := installed[key]; exists {
				continue
			}
			installed[key] = "0.0.1" // fallback, same as node agent
		}
	}

	return installed
}

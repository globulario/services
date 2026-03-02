package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	"google.golang.org/grpc/credentials/insecure"
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
	Short: "[DEPRECATED] Use 'globular services desired set' instead",
	Long: `DEPRECATED: Imperative install has been replaced by declarative desired-state management.

Use instead:
  globular services desired set <service> <version>
  globular services apply-desired

To continue using imperative install (unsupported), pass --dangerous-imperative.`,
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
	Short: "Seed the controller's desired state from installed services",
	Long: `Import all locally-installed services into the cluster controller's desired
state. This ensures that services installed by the installer or manually are
tracked by the controller and no longer appear as "Unmanaged".

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

// ─── adopt-installed ─────────────────────────────────────────────────────────

var servicesAdoptInstalledCmd = &cobra.Command{
	Use:   "adopt-installed",
	Short: "Import locally-installed services into the desired state",
	Long: `Alias for 'services seed'. Imports all locally-installed services into
the controller's desired state so they become declaratively managed.

Example:
  globular services adopt-installed --insecure`,
	RunE: runServicesSeed,
}

func init() {
	servicesApplyCmd.Flags().StringVar(&svcApplyService, "service", "", "Service name (required)")
	servicesApplyCmd.Flags().StringVar(&svcApplyVersion, "version", "", "Service version (required)")
	servicesApplyCmd.Flags().StringVar(&svcApplyPublisher, "publisher", "core@globular.io", "Publisher ID")
	servicesApplyCmd.Flags().StringVar(&svcApplyRepoAddr, "repository", "", "Repository gRPC endpoint (auto-discovered if empty)")
	servicesApplyCmd.Flags().BoolVar(&svcApplyRepoInsec, "repository-insecure", false, "Use plaintext for repository connection")
	servicesApplyCmd.Flags().BoolVar(&svcDangerousImperative, "dangerous-imperative", false, "Force legacy imperative install (unsupported)")
	_ = servicesApplyCmd.Flags().MarkHidden("dangerous-imperative")

	servicesApplyDesiredCmd.Flags().StringVar(&svcApplyPublisher, "publisher", "core@globular.io", "Publisher ID")
	servicesApplyDesiredCmd.Flags().StringVar(&svcApplyRepoAddr, "repository", "", "Repository gRPC endpoint (auto-discovered if empty)")
	servicesApplyDesiredCmd.Flags().BoolVar(&svcApplyRepoInsec, "repository-insecure", false, "Use plaintext for repository connection")

	servicesDesiredCmd.AddCommand(servicesDesiredSetCmd)
	servicesDesiredCmd.AddCommand(servicesDesiredRemoveCmd)
	servicesDesiredCmd.AddCommand(servicesDesiredListCmd)
	servicesDesiredCmd.AddCommand(servicesDesiredDiffCmd)

	servicesCmd.AddCommand(servicesApplyCmd)
	servicesCmd.AddCommand(servicesApplyDesiredCmd)
	servicesCmd.AddCommand(servicesSeedCmd)
	servicesCmd.AddCommand(servicesDesiredCmd)
	servicesCmd.AddCommand(servicesAdoptInstalledCmd)
}

// ─── apply ───────────────────────────────────────────────────────────────────

func runServicesApply(cmd *cobra.Command, args []string) error {
	if !svcDangerousImperative {
		return fmt.Errorf("imperative install has been removed\n\n" +
			"Use the declarative workflow instead:\n" +
			"  globular services desired set <service> <version>\n" +
			"  globular services apply-desired\n\n" +
			"To adopt existing installations into the desired state:\n" +
			"  globular services adopt-installed\n\n" +
			"To force legacy imperative install (unsupported), pass --dangerous-imperative")
	}

	resolveRepositoryAddr(cmd)

	if cv, err := versionutil.Canonical(svcApplyVersion); err == nil {
		svcApplyVersion = cv
	}

	fmt.Printf("→ Installing %s@%s …\n", svcApplyService, svcApplyVersion)

	tgzPath, err := downloadServiceArtifact(svcApplyService, svcApplyVersion, svcApplyPublisher)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tgzPath)

	unitName, err := installServiceTgz(tgzPath, svcApplyService)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	if unitName != "" {
		if err := systemctlActivate(unitName); err != nil {
			return fmt.Errorf("systemctl: %w", err)
		}
	}

	if err := writeVersionMarker(svcApplyService, svcApplyVersion); err != nil {
		return fmt.Errorf("version marker: %w", err)
	}

	fmt.Printf("OK: %s@%s installed and running\n", svcApplyService, svcApplyVersion)
	return nil
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

	var installed, skipped, adopted, failed int
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

		// If no version marker but the service is already running (e.g. installed
		// by globular-installer before version markers existed), adopt it by
		// writing the marker without re-downloading.
		if localVer == "" && isServiceRunning(name) {
			fmt.Printf("  ✓ %s@%s — already running, adopting\n", name, version)
			if err := writeVersionMarker(name, version); err != nil {
				fmt.Printf("    (version marker write failed: %v)\n", err)
			}
			adopted++
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

	fmt.Printf("\nSummary: %d installed, %d adopted, %d skipped (up-to-date), %d failed\n",
		installed, adopted, skipped, failed)
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
		fmt.Printf("%-40s %s\n", ds.GetServiceId(), ds.GetVersion())
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
		// Plaintext — build connection manually.
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
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

// isServiceRunning checks whether the systemd unit for the given service ID is
// active (running). The service ID may be fully qualified (e.g.
// "localhost/authentication") or a bare name ("authentication").
func isServiceRunning(serviceID string) bool {
	// Extract the base name: "localhost/authentication" → "authentication"
	base := serviceID
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	// Strip proto-style suffixes: "cluster_doctor.clusterdoctorservice" → "cluster_doctor"
	if idx := strings.Index(base, "."); idx > 0 {
		base = base[:idx]
	}
	// Normalize: underscores → hyphens
	base = strings.ReplaceAll(base, "_", "-")

	unit := "globular-" + base + ".service"
	out, err := exec.Command("systemctl", "is-active", unit).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
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
		discovered = ns.ControllerEndpoint
		isInsecure = ns.ControllerInsecure
		source = "node-agent state"
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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globularcli/pkgpack"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/resource/resourcepb"
)

// Exit codes for pkg publish (used with os.Exit).
const (
	exitPartial    = 1 // at least one package failed
	exitValidation = 2 // bad manifest or flags
	exitAuthRBAC   = 3 // authentication / RBAC error
)

var (
	pkgCmd = &cobra.Command{
		Use:   "pkg",
		Short: "Package build, verification and publishing",
	}

	pkgBuildCmd = &cobra.Command{
		Use:   "build",
		Short: "Build service packages from payload roots (installer assets or custom roots)",
		RunE:  runPkgBuild,
	}

	// pkg verify — kept for backward compatibility; pkg validate is the new name.
	pkgVerifyCmd = &cobra.Command{
		Use:   "verify",
		Short: "Verify a package tgz (alias for 'validate')",
		RunE:  runPkgValidate,
	}

	pkgValidateCmd = &cobra.Command{
		Use:   "validate",
		Short: "Validate package structure and manifest locally (no network)",
		Long: `Validate a package .tgz file locally without contacting any service.

Checks:
  • manifest.json present and schema valid
  • entrypoint exists inside the archive
  • platform field matches file name convention

Exit code 0 on success, 2 on validation error.`,
		RunE: runPkgValidate,
	}

	pkgDescribeCmd = &cobra.Command{
		Use:   "describe",
		Short: "Show parsed manifest fields from a package file",
		Long: `Parse and display the manifest embedded in a package .tgz.

Useful for debugging publisherID mismatches or verifying package metadata
before publishing.`,
		RunE: runPkgDescribe,
	}

	pkgRegisterCmd = &cobra.Command{
		Use:   "register",
		Short: "Register or update a package descriptor in ResourceService (no upload)",
		Long: `Register package metadata in ResourceService without uploading binaries.

Useful in CI/governance workflows where metadata registration and binary
upload are separate approval steps.

The descriptor is upserted (created or updated) using the caller's JWT token,
so RBAC applies correctly.`,
		RunE: runPkgRegister,
	}

	pkgPublishCmd = &cobra.Command{
		Use:   "publish",
		Short: "Publish a package to the repository service",
		Long: `Publish a package (.tgz) to the Globular repository service.

Full workflow:
  1. Parse manifest from the .tgz
  2. Upsert PackageDescriptor in ResourceService (with caller JWT → RBAC-correct)
  3. Upload bundle to RepositoryService

Authentication is required: run 'globular auth login' then 'globular auth install-certs'.

Examples:
  globular pkg publish --file service.echo_1.0.0_linux_amd64.tgz --repository localhost:10007
  globular pkg publish --dir ./packages --repository localhost:10007
  globular pkg publish --file pkg.tgz --repository localhost:10007 --output json | jq -e '.status=="success"'
`,
		RunE: runPkgPublish,
	}
)

var (
	pkgInstallerRoot      string
	pkgRoot               string
	pkgSpecPath           string
	pkgSpecDir            string
	pkgAssetsDir          string
	pkgBinDir             string
	pkgConfigDir          string
	pkgVersion            string
	pkgPublisher          string
	pkgPlatform           string
	pkgOutDir             string
	pkgSkipMissingConfig  bool
	pkgSkipMissingSystemd bool

	pkgVerifyFile string

	// Publish / register command flags
	pkgPublishFile       string
	pkgPublishDir        string
	pkgPublishRepository string
	pkgPublishPublisher  string
	pkgPublishDryRun     bool
	pkgPublishOutput     string // "table" | "json" | "yaml"

	// Register command flags (subset)
	pkgRegisterFile      string
	pkgRegisterName      string
	pkgRegisterVersion   string
	pkgRegisterType      string
	pkgRegisterPublisher string

	// Describe command flags
	pkgDescribeFile string
)

func init() {
	pkgCmd.AddCommand(pkgBuildCmd)
	pkgCmd.AddCommand(pkgVerifyCmd)
	pkgCmd.AddCommand(pkgValidateCmd)
	pkgCmd.AddCommand(pkgDescribeCmd)
	pkgCmd.AddCommand(pkgRegisterCmd)
	pkgCmd.AddCommand(pkgPublishCmd)

	pkgBuildCmd.Flags().StringVar(&pkgInstallerRoot, "installer-root", "", "path to globular-installer root")
	pkgBuildCmd.Flags().StringVar(&pkgRoot, "root", "", "payload root containing bin/ and config/")
	pkgBuildCmd.Flags().StringVar(&pkgSpecPath, "spec", "", "path to one YAML spec (exclusive with --spec-dir)")
	pkgBuildCmd.Flags().StringVar(&pkgSpecDir, "spec-dir", "", "directory of YAML specs")
	pkgBuildCmd.Flags().StringVar(&pkgAssetsDir, "assets", "", "assets directory (default resolved from installer-root)")
	pkgBuildCmd.Flags().StringVar(&pkgBinDir, "bin-dir", "", "explicit path to bin directory")
	pkgBuildCmd.Flags().StringVar(&pkgConfigDir, "config-dir", "", "explicit path to config directory")
	pkgBuildCmd.Flags().StringVar(&pkgVersion, "version", "", "package version (required)")
	pkgBuildCmd.Flags().StringVar(&pkgPublisher, "publisher", "core@globular.io", "publisher identifier")
	pkgBuildCmd.Flags().StringVar(&pkgPlatform, "platform", fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH), "target platform (goos_goarch)")
	pkgBuildCmd.Flags().StringVar(&pkgOutDir, "out", "", "output directory (required)")
	pkgBuildCmd.Flags().BoolVar(&pkgSkipMissingConfig, "skip-missing-config", true, "skip missing config directories")
	pkgBuildCmd.Flags().BoolVar(&pkgSkipMissingSystemd, "skip-missing-systemd", true, "skip missing systemd units")

	pkgVerifyCmd.Flags().StringVar(&pkgVerifyFile, "file", "", "path to a package tgz")
	pkgValidateCmd.Flags().StringVar(&pkgVerifyFile, "file", "", "path to a package tgz")

	pkgDescribeCmd.Flags().StringVar(&pkgDescribeFile, "file", "", "path to a package tgz (required)")

	pkgRegisterCmd.Flags().StringVar(&pkgRegisterFile, "file", "", "path to a package tgz (reads metadata from manifest)")
	pkgRegisterCmd.Flags().StringVar(&pkgRegisterName, "name", "", "package name (required when --file not given)")
	pkgRegisterCmd.Flags().StringVar(&pkgRegisterVersion, "version", "", "package version (required when --file not given)")
	pkgRegisterCmd.Flags().StringVar(&pkgRegisterType, "type", "service", "package type: service|application")
	pkgRegisterCmd.Flags().StringVar(&pkgRegisterPublisher, "publisher", "", "publisher ID (overrides manifest)")

	pkgPublishCmd.Flags().StringVar(&pkgPublishFile, "file", "", "path to a package tgz to publish")
	pkgPublishCmd.Flags().StringVar(&pkgPublishDir, "dir", "", "directory containing package tgz files to publish")
	pkgPublishCmd.Flags().StringVar(&pkgPublishRepository, "repository", "", "repository service address (required)")
	pkgPublishCmd.Flags().StringVar(&pkgPublishPublisher, "publisher", "", "override publisher from package manifest")
	pkgPublishCmd.Flags().BoolVar(&pkgPublishDryRun, "dry-run", false, "validate packages without uploading")
	pkgPublishCmd.Flags().StringVar(&pkgPublishOutput, "output", "table", "output format: table|json|yaml")
}

// ── Build ──────────────────────────────────────────────────────────────────

func runPkgBuild(cmd *cobra.Command, args []string) error {
	if (pkgSpecPath == "" && pkgSpecDir == "") || (pkgSpecPath != "" && pkgSpecDir != "") {
		return errors.New("set either --spec or --spec-dir")
	}
	if pkgVersion == "" {
		return errors.New("--version is required")
	}
	if pkgOutDir == "" {
		return errors.New("--out is required")
	}

	rootMode := pkgRoot != ""
	explicitMode := pkgBinDir != "" || pkgConfigDir != ""
	installerMode := pkgInstallerRoot != "" || pkgAssetsDir != ""
	modeCount := 0
	for _, active := range []bool{rootMode, explicitMode, installerMode} {
		if active {
			modeCount++
		}
	}
	if modeCount == 0 {
		return errors.New("one of --installer-root/--assets, --root, or --bin-dir+--config-dir is required")
	}
	if modeCount > 1 {
		return errors.New("choose only one of --installer-root/--assets, --root, or --bin-dir+--config-dir")
	}
	if explicitMode && (pkgBinDir == "" || pkgConfigDir == "") {
		return errors.New("--bin-dir and --config-dir must both be set when using explicit paths")
	}

	results, err := pkgpack.BuildPackages(pkgpack.BuildOptions{
		InstallerRoot:      pkgInstallerRoot,
		Root:               pkgRoot,
		SpecPath:           pkgSpecPath,
		SpecDir:            pkgSpecDir,
		AssetsDir:          pkgAssetsDir,
		BinDir:             pkgBinDir,
		ConfigDir:          pkgConfigDir,
		Version:            pkgVersion,
		Publisher:          pkgPublisher,
		Platform:           pkgPlatform,
		OutDir:             pkgOutDir,
		SkipMissingConfig:  pkgSkipMissingConfig,
		SkipMissingSystemd: pkgSkipMissingSystemd,
	})
	printPkgBuildSummary(results)
	return err
}

func printPkgBuildSummary(results []pkgpack.BuildResult) {
	if len(results) == 0 {
		fmt.Println("no packages built")
		return
	}
	fmt.Println("summary:")
	for _, res := range results {
		name := res.Service
		if name == "" {
			name = res.SpecPath
		}
		if res.Err != nil {
			fmt.Printf("[FAIL] %s: %v\n", name, res.Err)
		} else {
			fmt.Printf("[OK] %s -> %s\n", name, res.OutputPath)
		}
	}
}

// ── Validate / Verify ──────────────────────────────────────────────────────

func runPkgValidate(cmd *cobra.Command, args []string) error {
	if pkgVerifyFile == "" {
		return errors.New("--file is required")
	}
	summary, err := pkgpack.VerifyTGZ(pkgVerifyFile)
	if err != nil {
		return err
	}
	fmt.Printf("verified: name=%s version=%s platform=%s entrypoint=%s configs=%d systemd=%d file=%s\n",
		summary.Name, summary.Version, summary.Platform, summary.Entrypoint,
		summary.ConfigCount, summary.SystemdCount, pkgVerifyFile)
	return nil
}

// ── Describe ───────────────────────────────────────────────────────────────

func runPkgDescribe(cmd *cobra.Command, args []string) error {
	if pkgDescribeFile == "" {
		return errors.New("--file is required")
	}
	summary, err := pkgpack.VerifyTGZ(pkgDescribeFile)
	if err != nil {
		return err
	}
	switch strings.ToLower(rootCfg.output) {
	case "json":
		type descJSON struct {
			Name         string `json:"name"`
			Version      string `json:"version"`
			Platform     string `json:"platform"`
			Publisher    string `json:"publisher"`
			Entrypoint   string `json:"entrypoint"`
			ConfigCount  int    `json:"config_count"`
			SystemdCount int    `json:"systemd_count"`
			File         string `json:"file"`
		}
		b, _ := json.MarshalIndent(descJSON{
			Name:         summary.Name,
			Version:      summary.Version,
			Platform:     summary.Platform,
			Publisher:    summary.Publisher,
			Entrypoint:   summary.Entrypoint,
			ConfigCount:  summary.ConfigCount,
			SystemdCount: summary.SystemdCount,
			File:         pkgDescribeFile,
		}, "", "  ")
		fmt.Println(string(b))
	default:
		fmt.Printf("%-14s: %s\n", "Name", summary.Name)
		fmt.Printf("%-14s: %s\n", "Version", summary.Version)
		fmt.Printf("%-14s: %s\n", "Platform", summary.Platform)
		fmt.Printf("%-14s: %s\n", "Publisher", summary.Publisher)
		fmt.Printf("%-14s: %s\n", "Entrypoint", summary.Entrypoint)
		fmt.Printf("%-14s: %d\n", "Configs", summary.ConfigCount)
		fmt.Printf("%-14s: %d\n", "Systemd units", summary.SystemdCount)
		fmt.Printf("%-14s: %s\n", "File", pkgDescribeFile)
	}
	return nil
}

// ── Register ───────────────────────────────────────────────────────────────

func runPkgRegister(cmd *cobra.Command, args []string) error {
	token := rootCfg.token
	if token == "" {
		token = os.Getenv("GLOBULAR_TOKEN")
	}
	if token == "" {
		return errors.New("authentication required: run 'globular auth login' or provide --token")
	}

	var name, version, publisher string

	if pkgRegisterFile != "" {
		summary, err := pkgpack.VerifyTGZ(pkgRegisterFile)
		if err != nil {
			return fmt.Errorf("read package: %w", err)
		}
		name = summary.Name
		version = summary.Version
		publisher = summary.Publisher
	}

	// Flags override manifest values.
	if pkgRegisterName != "" {
		name = pkgRegisterName
	}
	if pkgRegisterVersion != "" {
		version = pkgRegisterVersion
	}
	if pkgRegisterPublisher != "" {
		publisher = pkgRegisterPublisher
	}
	if publisher == "" {
		publisher = "core@globular.io"
	}

	if name == "" {
		return errors.New("package name required: provide --file or --name")
	}
	if version == "" {
		return errors.New("package version required: provide --file or --version")
	}

	pkgType := resourcepb.PackageType_SERVICE_TYPE
	if strings.ToLower(pkgRegisterType) == "application" {
		pkgType = resourcepb.PackageType_APPLICATION_TYPE
	}

	action, err := setPackageDescriptor(name, publisher, version, pkgType)
	if err != nil {
		return err
	}
	fmt.Printf("descriptor %s: name=%s version=%s publisher=%s\n", action, name, version, publisher)
	return nil
}

// ── Publish ────────────────────────────────────────────────────────────────

func runPkgPublish(cmd *cobra.Command, args []string) error {
	// Flag validation — exit 2 on bad input.
	if pkgPublishFile == "" && pkgPublishDir == "" {
		fmt.Fprintln(os.Stderr, "Error: either --file or --dir is required")
		os.Exit(exitValidation)
	}
	if pkgPublishFile != "" && pkgPublishDir != "" {
		fmt.Fprintln(os.Stderr, "Error: use either --file or --dir, not both")
		os.Exit(exitValidation)
	}
	if pkgPublishRepository == "" {
		fmt.Fprintln(os.Stderr, "Error: --repository is required")
		os.Exit(exitValidation)
	}

	token := rootCfg.token
	if token == "" {
		token = os.Getenv("GLOBULAR_TOKEN")
	}
	if token == "" && !pkgPublishDryRun {
		fmt.Fprintln(os.Stderr, "Error: authentication required: run 'globular auth login' or provide --token")
		os.Exit(exitAuthRBAC)
	}

	// Collect files.
	var files []string
	if pkgPublishFile != "" {
		files = []string{pkgPublishFile}
	} else {
		entries, err := os.ReadDir(pkgPublishDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading directory: %v\n", err)
			os.Exit(exitValidation)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tgz") {
				files = append(files, filepath.Join(pkgPublishDir, entry.Name()))
			}
		}
		if len(files) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no .tgz files found in directory")
			os.Exit(exitValidation)
		}
	}

	dirMode := len(files) > 1 || pkgPublishDir != ""
	start := time.Now()

	if !dirMode {
		// Single-file mode.
		result := publishOne(files[0], token)
		out := singlePublishOutput(result)
		if err := renderPkgPublish(out, pkgPublishOutput); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		if result.authErr {
			os.Exit(exitAuthRBAC)
		}
		if result.err != nil {
			os.Exit(exitPartial)
		}
		return nil
	}

	// Directory mode.
	var perPkg []pkgPublishOne
	for _, f := range files {
		perPkg = append(perPkg, publishOne(f, token))
	}
	out := dirPublishOutput(perPkg, time.Since(start))
	if err := renderPkgPublish(out, pkgPublishOutput); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	for _, r := range perPkg {
		if r.authErr {
			os.Exit(exitAuthRBAC)
		}
	}
	for _, r := range perPkg {
		if r.err != nil {
			os.Exit(exitPartial)
		}
	}
	return nil
}

// pkgPublishOne is the internal result of publishing a single package.
type pkgPublishOne struct {
	file             string
	name             string
	version          string
	platform         string
	publisher        string
	digest           string
	bundleID         string
	descriptorAction string
	sizeBytes        int64
	duration         time.Duration
	err              error
	authErr          bool // true when err is auth/RBAC
}

func publishOne(file, token string) pkgPublishOne {
	start := time.Now()
	r := pkgPublishOne{file: file}

	summary, err := pkgpack.VerifyTGZ(file)
	if err != nil {
		r.err = fmt.Errorf("validation failed: %w", err)
		r.duration = time.Since(start)
		return r
	}
	r.name = summary.Name
	r.version = summary.Version
	r.platform = summary.Platform

	publisher := pkgPublishPublisher
	if publisher == "" {
		publisher = summary.Publisher
	}
	if publisher == "" {
		publisher = "core@globular.io"
	}
	r.publisher = publisher

	if pkgPublishDryRun {
		r.duration = time.Since(start)
		return r
	}

	// Preflight: mTLS credentials required.
	if _, err := getTLSCredentialsWithOptions(true); err != nil {
		r.err = err
		r.authErr = true
		r.duration = time.Since(start)
		return r
	}

	// Step A: upsert descriptor under user identity.
	action, err := setPackageDescriptor(summary.Name, publisher, summary.Version, resourcepb.PackageType_SERVICE_TYPE)
	if err != nil {
		r.err = err
		if isAuthErr(err) {
			r.authErr = true
		}
		r.duration = time.Since(start)
		return r
	}
	r.descriptorAction = action

	// Step B: compute digest before upload.
	r.digest = pkgSHA256(file)

	// Step C: upload bundle.
	client, err := repository_client.NewRepositoryService_Client(pkgPublishRepository, "repository.PackageRepository")
	if err != nil {
		r.err = fmt.Errorf("connect to repository: %w", err)
		r.duration = time.Since(start)
		return r
	}
	defer client.Close()

	size, err := client.UploadBundle(
		token,
		pkgPublishRepository,
		summary.Name,
		publisher,
		summary.Version,
		summary.Platform,
		file,
	)
	if err != nil {
		r.err = fmt.Errorf("upload failed: %w", err)
		if isAuthErr(err) {
			r.authErr = true
		}
		r.duration = time.Since(start)
		return r
	}

	r.sizeBytes = int64(size)
	r.bundleID = pkgBundleID(summary.Name, summary.Version, summary.Platform)
	r.duration = time.Since(start)
	return r
}

func singlePublishOutput(r pkgPublishOne) *PkgPublishOutput {
	out := &PkgPublishOutput{
		Package: PkgPublishPackage{
			Name:      r.name,
			Version:   r.version,
			Platform:  r.platform,
			Publisher: r.publisher,
		},
		Repository: pkgPublishRepository,
		DurationMS: pkgMillis(r.duration),
	}
	if r.err != nil {
		out.Status = "failed"
		out.DescriptorAction = r.descriptorAction // may be set even if upload fails
		out.Error = pkgPublishErrorFrom(r.err)
	} else if pkgPublishDryRun {
		out.Status = "dry-run"
	} else {
		out.Status = "success"
		out.DescriptorAction = r.descriptorAction
		out.BundleID = r.bundleID
		out.Digest = r.digest
		out.SizeBytes = r.sizeBytes
	}
	return out
}

func dirPublishOutput(results []pkgPublishOne, total time.Duration) *PkgPublishOutput {
	var succeeded, failed int
	var perPkg []PkgPublishResult
	for _, r := range results {
		pr := PkgPublishResult{
			Name:       r.name,
			Version:    r.version,
			Platform:   r.platform,
			Publisher:  r.publisher,
			Repository: pkgPublishRepository,
			DurationMS: pkgMillis(r.duration),
		}
		if r.err != nil {
			pr.Status = "failed"
			pr.DescriptorAction = r.descriptorAction
			pr.Error = pkgPublishErrorFrom(r.err)
			failed++
		} else {
			pr.Status = "success"
			pr.DescriptorAction = r.descriptorAction
			pr.BundleID = r.bundleID
			pr.Digest = r.digest
			pr.SizeBytes = r.sizeBytes
			succeeded++
		}
		perPkg = append(perPkg, pr)
	}
	return &PkgPublishOutput{
		Summary: &PkgPublishSummary{
			Total:      len(results),
			Succeeded:  succeeded,
			Failed:     failed,
			DurationMS: pkgMillis(total),
		},
		Results: perPkg,
	}
}

func pkgPublishErrorFrom(err error) *PkgPublishError {
	if err == nil {
		return nil
	}
	code := "Internal"
	if st, ok := status.FromError(err); ok {
		code = st.Code().String()
	}
	return &PkgPublishError{Code: code, Message: err.Error()}
}

func isAuthErr(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		c := st.Code()
		return c == codes.Unauthenticated || c == codes.PermissionDenied
	}
	// Check wrapped sentinel
	return errors.Is(err, ErrNeedInstallCerts)
}

// ── Descriptor upsert ──────────────────────────────────────────────────────

// defaultResourcePort is the fallback used when service discovery is unavailable.
const defaultResourcePort = 10010

// setPackageDescriptor calls ResourceService.SetPackageDescriptor with the
// caller's JWT (injected by dialGRPC) so RBAC applies under user identity.
//
// It probes with GetPackageDescriptor first to distinguish "created" vs
// "updated", falling back to "upserted" when the probe itself errors.
//
// Returns the descriptor action ("created"|"updated"|"upserted") and any error.
func setPackageDescriptor(name, publisherID, version string, pkgType resourcepb.PackageType) (string, error) {
	addr := config.ResolveServiceAddr(
		"resource.ResourceService",
		fmt.Sprintf("localhost:%d", defaultResourcePort),
	)
	conn, err := dialGRPC(addr)
	if err != nil {
		return "", fmt.Errorf("connect to resource service: %w", err)
	}
	defer conn.Close()

	rc := resourcepb.NewResourceServiceClient(conn)

	// Probe: does the descriptor already exist?
	action := "upserted"
	_, probeErr := rc.GetPackageDescriptor(ctxWithTimeout(), &resourcepb.GetPackageDescriptorRequest{
		ServiceId:   name,
		PublisherID: publisherID,
	})
	if probeErr != nil {
		if st, ok := status.FromError(probeErr); ok && st.Code() == codes.NotFound {
			action = "created"
		}
		// Any other probe error: still attempt the upsert; action stays "upserted".
	} else {
		action = "updated"
	}

	desc := &resourcepb.PackageDescriptor{
		Id:          name,
		Name:        name,
		Type:        pkgType,
		PublisherID: publisherID,
		Version:     version,
	}
	_, err = rc.SetPackageDescriptor(ctxWithTimeout(), &resourcepb.SetPackageDescriptorRequest{
		PackageDescriptor: desc,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			c := st.Code()
			if c == codes.PermissionDenied || c == codes.Unauthenticated {
				return "", fmt.Errorf("publish denied: missing role repo.publisher for publisher %q: %w", publisherID, err)
			}
		}
		return "", fmt.Errorf("register package descriptor: %w", err)
	}
	return action, nil
}

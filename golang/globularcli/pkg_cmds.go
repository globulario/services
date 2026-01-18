package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/globularcli/pkgpack"
	"github.com/globulario/services/golang/repository/repository_client"
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

	pkgVerifyCmd = &cobra.Command{
		Use:   "verify",
		Short: "Verify a package tgz",
		RunE:  runPkgVerify,
	}

	pkgPublishCmd = &cobra.Command{
		Use:   "publish",
		Short: "Publish a package to the repository service",
		Long: `Publish a package (.tgz) to the Globular repository service.

The package must be a valid .tgz file created by 'globular pkg build'.
Authentication is done via the --token flag (global) or GLOBULAR_TOKEN env var.

Examples:
  # Publish a single package
  globular pkg publish --file service.echo_1.0.0_linux_amd64.tgz --repository localhost:10003

  # Publish all packages in a directory
  globular pkg publish --dir ./packages --repository localhost:10003

  # Publish with custom publisher
  globular pkg publish --file myservice.tgz --repository localhost:10003 --publisher myorg@example.com
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

	// Publish command flags
	pkgPublishFile       string
	pkgPublishDir        string
	pkgPublishRepository string
	pkgPublishPublisher  string
	pkgPublishDryRun     bool
)

func init() {
	pkgCmd.AddCommand(pkgBuildCmd)
	pkgCmd.AddCommand(pkgVerifyCmd)
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

	// Publish command flags
	pkgPublishCmd.Flags().StringVar(&pkgPublishFile, "file", "", "path to a package tgz to publish")
	pkgPublishCmd.Flags().StringVar(&pkgPublishDir, "dir", "", "directory containing package tgz files to publish")
	pkgPublishCmd.Flags().StringVar(&pkgPublishRepository, "repository", "", "repository service address (required)")
	pkgPublishCmd.Flags().StringVar(&pkgPublishPublisher, "publisher", "", "override publisher from package manifest")
	pkgPublishCmd.Flags().BoolVar(&pkgPublishDryRun, "dry-run", false, "validate packages without uploading")
}

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

func runPkgVerify(cmd *cobra.Command, args []string) error {
	if pkgVerifyFile == "" {
		return errors.New("--file is required")
	}
	summary, err := pkgpack.VerifyTGZ(pkgVerifyFile)
	if err != nil {
		return err
	}
	fmt.Printf("verified: name=%s version=%s platform=%s entrypoint=%s configs=%d systemd=%d file=%s\n",
		summary.Name, summary.Version, summary.Platform, summary.Entrypoint, summary.ConfigCount, summary.SystemdCount, pkgVerifyFile)
	return nil
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

// PublishResult holds the outcome of a single package publish attempt.
type PublishResult struct {
	File    string
	Name    string
	Version string
	Size    int
	Err     error
}

func runPkgPublish(cmd *cobra.Command, args []string) error {
	// Validate flags
	if pkgPublishFile == "" && pkgPublishDir == "" {
		return errors.New("either --file or --dir is required")
	}
	if pkgPublishFile != "" && pkgPublishDir != "" {
		return errors.New("use either --file or --dir, not both")
	}
	if pkgPublishRepository == "" {
		return errors.New("--repository is required")
	}

	// Get token from flag or environment
	token := rootCfg.token
	if token == "" {
		token = os.Getenv("GLOBULAR_TOKEN")
	}
	if token == "" && !pkgPublishDryRun {
		return errors.New("authentication required: set --token flag or GLOBULAR_TOKEN environment variable")
	}

	// Collect package files to publish
	var files []string
	if pkgPublishFile != "" {
		files = []string{pkgPublishFile}
	} else {
		entries, err := os.ReadDir(pkgPublishDir)
		if err != nil {
			return fmt.Errorf("reading directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tgz") {
				files = append(files, filepath.Join(pkgPublishDir, entry.Name()))
			}
		}
		if len(files) == 0 {
			return errors.New("no .tgz files found in directory")
		}
	}

	// Process each package
	var results []PublishResult
	for _, file := range files {
		result := publishPackage(file, token)
		results = append(results, result)
	}

	// Print summary
	printPublishSummary(results)

	// Return error if any failed
	for _, r := range results {
		if r.Err != nil {
			return errors.New("one or more packages failed to publish")
		}
	}
	return nil
}

func publishPackage(file, token string) PublishResult {
	result := PublishResult{File: file}

	// Verify the package first
	summary, err := pkgpack.VerifyTGZ(file)
	if err != nil {
		result.Err = fmt.Errorf("verification failed: %w", err)
		return result
	}

	result.Name = summary.Name
	result.Version = summary.Version

	// Get file size
	info, err := os.Stat(file)
	if err != nil {
		result.Err = fmt.Errorf("stat failed: %w", err)
		return result
	}

	if pkgPublishDryRun {
		fmt.Printf("[DRY-RUN] would publish: %s (name=%s version=%s platform=%s size=%d)\n",
			file, summary.Name, summary.Version, summary.Platform, info.Size())
		return result
	}

	// Create repository client
	client, err := repository_client.NewRepositoryService_Client(pkgPublishRepository, "repository.PackageRepository")
	if err != nil {
		result.Err = fmt.Errorf("connect to repository: %w", err)
		return result
	}
	defer client.Close()

	// Determine publisher
	publisher := pkgPublishPublisher
	if publisher == "" {
		publisher = summary.Publisher
	}
	if publisher == "" {
		publisher = "core@globular.io"
	}

	// Upload the bundle
	fmt.Printf("publishing %s (name=%s version=%s platform=%s)...\n",
		filepath.Base(file), summary.Name, summary.Version, summary.Platform)

	size, err := client.UploadBundle(
		token,
		pkgPublishRepository, // discoveryId (same as repository address)
		summary.Name,         // serviceId
		publisher,            // PublisherID
		summary.Version,      // version
		summary.Platform,     // platform
		file,                 // packagePath
	)
	if err != nil {
		result.Err = fmt.Errorf("upload failed: %w", err)
		return result
	}

	result.Size = size
	return result
}

func printPublishSummary(results []PublishResult) {
	if len(results) == 0 {
		fmt.Println("no packages to publish")
		return
	}

	fmt.Println("\npublish summary:")
	var succeeded, failed int
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("  [FAIL] %s: %v\n", filepath.Base(r.File), r.Err)
			failed++
		} else if pkgPublishDryRun {
			fmt.Printf("  [DRY-RUN] %s\n", filepath.Base(r.File))
			succeeded++
		} else {
			fmt.Printf("  [OK] %s (%s v%s, %d bytes uploaded)\n",
				filepath.Base(r.File), r.Name, r.Version, r.Size)
			succeeded++
		}
	}
	fmt.Printf("\ntotal: %d succeeded, %d failed\n", succeeded, failed)
}

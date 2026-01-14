package main

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/globularcli/pkgpack"
)

var (
	pkgCmd = &cobra.Command{
		Use:   "pkg",
		Short: "Package build and verification",
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
)

func init() {
	pkgCmd.AddCommand(pkgBuildCmd)
	pkgCmd.AddCommand(pkgVerifyCmd)

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

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
		Short: "Build service packages from installer assets/specs",
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
	pkgSpecPath           string
	pkgSpecDir            string
	pkgAssetsDir          string
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
	pkgBuildCmd.Flags().StringVar(&pkgSpecPath, "spec", "", "path to one YAML spec (exclusive with --spec-dir)")
	pkgBuildCmd.Flags().StringVar(&pkgSpecDir, "spec-dir", "", "directory of YAML specs")
	pkgBuildCmd.Flags().StringVar(&pkgAssetsDir, "assets", "", "assets directory (default resolved from installer-root)")
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
	if pkgInstallerRoot == "" && pkgAssetsDir == "" {
		return errors.New("--installer-root is required (or specify --assets and absolute specs)")
	}

	results, err := pkgpack.BuildPackages(pkgpack.BuildOptions{
		InstallerRoot:      pkgInstallerRoot,
		SpecPath:           pkgSpecPath,
		SpecDir:            pkgSpecDir,
		AssetsDir:          pkgAssetsDir,
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

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	supportBundleOut      string
	supportBundleSince    string
	supportBundleServices []string
)

var supportBundleCmd = &cobra.Command{
	Use:   "support-bundle",
	Short: "Collect diagnostics into a support bundle",
	Long: `Collect system diagnostics, logs, and configuration into a compressed archive.

The support bundle includes:
  - systemctl status for all Globular services
  - journald logs for services (with --since filter)
  - Network configuration (/var/lib/globular/network.json)
  - Globular configuration files (/etc/globular/*)
  - Envoy config dump (if available)
  - Service versions and git SHAs

Examples:
  globular support-bundle
  globular support-bundle --out /tmp/diagnostics.tar.gz
  globular support-bundle --since 2h
  globular support-bundle --services gateway,xds,envoy
`,
	RunE: runSupportBundle,
}

func init() {
	supportBundleCmd.Flags().StringVar(&supportBundleOut, "out", "", "Output path for the bundle (default: support-bundle-<timestamp>.tar.gz)")
	supportBundleCmd.Flags().StringVar(&supportBundleSince, "since", "24h", "Collect logs since this duration (e.g., 2h, 30m, 1d)")
	supportBundleCmd.Flags().StringSliceVar(&supportBundleServices, "services", nil, "Specific services to include (default: all)")

	clusterCmd.AddCommand(supportBundleCmd)
}

// supportBundleCollector manages the collection of diagnostics
type supportBundleCollector struct {
	outputPath  string
	tempDir     string
	tarWriter   *tar.Writer
	gzipWriter  *gzip.Writer
	fileHandle  *os.File
	sinceFilter string
	services    []string
}

func runSupportBundle(cmd *cobra.Command, args []string) error {
	// Determine output path
	outputPath := supportBundleOut
	if outputPath == "" {
		timestamp := time.Now().Format("20060102-150405")
		outputPath = fmt.Sprintf("support-bundle-%s.tar.gz", timestamp)
	}

	// Create temporary directory for collecting files
	tempDir, err := os.MkdirTemp("", "support-bundle-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	collector := &supportBundleCollector{
		outputPath:  outputPath,
		tempDir:     tempDir,
		sinceFilter: supportBundleSince,
		services:    supportBundleServices,
	}

	fmt.Printf("Collecting diagnostics into %s...\n", outputPath)

	// Collect all diagnostics
	if err := collector.collect(); err != nil {
		return fmt.Errorf("collect diagnostics: %w", err)
	}

	fmt.Printf("✅ Support bundle created: %s\n", outputPath)
	return nil
}

func (c *supportBundleCollector) collect() error {
	// Create output file
	file, err := os.Create(c.outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer file.Close()

	c.fileHandle = file
	c.gzipWriter = gzip.NewWriter(file)
	defer c.gzipWriter.Close()

	c.tarWriter = tar.NewWriter(c.gzipWriter)
	defer c.tarWriter.Close()

	// Create root directory in archive
	rootDir := "support-bundle"

	// Collect metadata
	if err := c.collectMetadata(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to collect metadata: %v\n", err)
	}

	// Collect systemctl status
	if err := c.collectSystemctlStatus(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to collect systemctl status: %v\n", err)
	}

	// Collect journald logs
	if err := c.collectJournaldLogs(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to collect journald logs: %v\n", err)
	}

	// Collect network spec
	if err := c.collectNetworkSpec(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to collect network spec: %v\n", err)
	}

	// Collect configs
	if err := c.collectConfigs(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to collect configs: %v\n", err)
	}

	// Collect envoy config dump
	if err := c.collectEnvoyConfig(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to collect envoy config: %v\n", err)
	}

	// Collect versions
	if err := c.collectVersions(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to collect versions: %v\n", err)
	}

	return nil
}

func (c *supportBundleCollector) collectMetadata(rootDir string) error {
	metadata := map[string]interface{}{
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"hostname":      getHostname(),
		"since_filter":  c.sinceFilter,
		"services":      c.services,
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return c.addFileFromBytes(filepath.Join(rootDir, "metadata.json"), data)
}

func (c *supportBundleCollector) collectSystemctlStatus(rootDir string) error {
	services := c.getServiceList()
	statusDir := filepath.Join(rootDir, "systemctl-status")

	for _, service := range services {
		unitName := service + ".service"
		output, _ := exec.Command("systemctl", "status", unitName).CombinedOutput()

		// Always include output, even if command failed
		filename := filepath.Join(statusDir, service+"-status.txt")
		if err := c.addFileFromBytes(filename, output); err != nil {
			return err
		}
	}

	return nil
}

func (c *supportBundleCollector) collectJournaldLogs(rootDir string) error {
	services := c.getServiceList()
	logsDir := filepath.Join(rootDir, "logs")

	for _, service := range services {
		unitName := service + ".service"
		args := []string{"-u", unitName, "--no-pager"}

		if c.sinceFilter != "" {
			args = append(args, "--since", c.sinceFilter)
		}

		output, _ := exec.Command("journalctl", args...).CombinedOutput()

		// Always include output, even if command failed or empty
		filename := filepath.Join(logsDir, service+".log")
		if err := c.addFileFromBytes(filename, output); err != nil {
			return err
		}
	}

	return nil
}

func (c *supportBundleCollector) collectNetworkSpec(rootDir string) error {
	networkPath := "/var/lib/globular/network.json"
	if data, err := os.ReadFile(networkPath); err == nil {
		return c.addFileFromBytes(filepath.Join(rootDir, "configs", "network.json"), data)
	} else {
		// Include error note
		errNote := fmt.Sprintf("Error reading network spec: %v\n", err)
		return c.addFileFromBytes(filepath.Join(rootDir, "configs", "network.json.error"), []byte(errNote))
	}
}

func (c *supportBundleCollector) collectConfigs(rootDir string) error {
	configsDir := "/etc/globular"
	outputDir := filepath.Join(rootDir, "configs", "etc-globular")

	// Walk through /etc/globular
	return filepath.Walk(configsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Can't access path, note it
			errNote := fmt.Sprintf("Error accessing %s: %v\n", path, err)
			errorPath := filepath.Join(outputDir, "access-errors.txt")
			return c.addFileFromBytes(errorPath, []byte(errNote))
		}

		if info.IsDir() {
			return nil
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			errNote := fmt.Sprintf("Error reading %s: %v\n", path, err)
			return c.addFileFromBytes(filepath.Join(outputDir, "read-errors.txt"), []byte(errNote))
		}

		// Sanitize sensitive data (basic check for keywords)
		if c.containsSensitiveData(path) {
			data = []byte(fmt.Sprintf("# File %s excluded (may contain sensitive data)\n", path))
		}

		// Determine relative path
		relPath, err := filepath.Rel(configsDir, path)
		if err != nil {
			relPath = filepath.Base(path)
		}

		return c.addFileFromBytes(filepath.Join(outputDir, relPath), data)
	})
}

func (c *supportBundleCollector) collectEnvoyConfig(rootDir string) error {
	// Try to fetch envoy config dump from admin interface
	output, err := exec.Command("curl", "-s", "http://localhost:9901/config_dump").CombinedOutput()
	if err != nil {
		errNote := fmt.Sprintf("Error fetching envoy config: %v\n", err)
		return c.addFileFromBytes(filepath.Join(rootDir, "envoy", "config_dump.error"), []byte(errNote))
	}

	return c.addFileFromBytes(filepath.Join(rootDir, "envoy", "config_dump.json"), output)
}

func (c *supportBundleCollector) collectVersions(rootDir string) error {
	versions := map[string]string{}

	// Try to get git SHA if available
	if output, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		versions["git_sha"] = strings.TrimSpace(string(output))
	}

	// Try to get binary versions (if they support --version)
	binaries := []string{"globular", "globular-gateway", "globular-xds"}
	for _, binary := range binaries {
		if output, err := exec.Command(binary, "--version").Output(); err == nil {
			versions[binary] = strings.TrimSpace(string(output))
		}
	}

	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return err
	}

	return c.addFileFromBytes(filepath.Join(rootDir, "versions.json"), data)
}

func (c *supportBundleCollector) addFileFromBytes(name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}

	if err := c.tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := c.tarWriter.Write(data); err != nil {
		return err
	}

	return nil
}

func (c *supportBundleCollector) getServiceList() []string {
	if len(c.services) > 0 {
		return c.services
	}

	// Default list of Globular services
	return []string{
		"globular-gateway",
		"globular-xds",
		"envoy",
		"etcd",
		"scylla",
		"minio",
		"globular-dns",
		"globular-nodeagent",
		"globular-clustercontroller",
	}
}

func (c *supportBundleCollector) containsSensitiveData(path string) bool {
	// Basic check for files that might contain sensitive data
	lowerPath := strings.ToLower(path)
	sensitiveKeywords := []string{"secret", "password", "key", "token", "credential"}

	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}

	return false
}

func getHostname() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "unknown"
}

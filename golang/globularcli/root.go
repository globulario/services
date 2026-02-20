package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var rootCfg = struct {
	controllerAddr string
	nodeAddr       string
	dnsAddr        string
	token          string
	caFile         string
	insecure       bool
	timeout        time.Duration
	output         string
}{
	controllerAddr: "localhost:10000",
	nodeAddr:       "localhost:11000",
	dnsAddr:        "localhost:10006", // Updated from 10033 to actual DNS service port
	timeout:        5 * time.Second,
	output:         "table",
}

var rootCmd = &cobra.Command{
	Use:   "globular",
	Short: "Globular control-plane CLI",
	// Auto-load cached token when --token is not explicitly provided.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Resolve CA certificate and export as GLOBULAR_CA_CERT so that all
		// service client code (InitClient → GetEtcdTLS → GetCACertificatePath)
		// finds the right CA regardless of install layout.
		//
		// Priority: --ca flag → GLOBULAR_CA_CERT env → user PKI → legacy tls/ path → system CA.
		if rootCfg.caFile != "" {
			if _, err := os.Stat(rootCfg.caFile); err != nil {
				return fmt.Errorf("--ca: %w", err)
			}
			_ = os.Setenv("GLOBULAR_CA_CERT", rootCfg.caFile)
		} else if os.Getenv("GLOBULAR_CA_CERT") == "" {
			// Not explicitly provided — try to resolve automatically and export
			// so downstream library code (config.GetCACertificatePath) can use it.
			if caPath, err := resolveCAPath(); err == nil {
				_ = os.Setenv("GLOBULAR_CA_CERT", caPath)
			}
			// Failure is non-fatal here; individual commands will surface the error
			// when they actually attempt a TLS connection.
		}

		if rootCfg.token == "" {
			home := os.Getenv("HOME")
			if home == "" {
				home, _ = os.UserHomeDir()
			}
			tokenFile := filepath.Join(home, ".config", "globular", "token")
			if data, err := os.ReadFile(tokenFile); err == nil {
				rootCfg.token = strings.TrimSpace(string(data))
			}
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootCfg.controllerAddr, "controller", rootCfg.controllerAddr, "Cluster controller gRPC endpoint")
	rootCmd.PersistentFlags().StringVar(&rootCfg.nodeAddr, "node", rootCfg.nodeAddr, "Node agent gRPC endpoint")
	rootCmd.PersistentFlags().StringVar(&rootCfg.dnsAddr, "dns", rootCfg.dnsAddr, "DNS service gRPC endpoint")
	rootCmd.PersistentFlags().StringVar(&rootCfg.token, "token", "", "Authorization token for the control plane")
	rootCmd.PersistentFlags().BoolVar(&rootCfg.insecure, "insecure", false, "Skip TLS verification")
	rootCmd.PersistentFlags().StringVar(&rootCfg.caFile, "ca", "", "Path to CA bundle")
	rootCmd.PersistentFlags().DurationVar(&rootCfg.timeout, "timeout", rootCfg.timeout, "Request timeout")
	rootCmd.PersistentFlags().StringVar(&rootCfg.output, "output", rootCfg.output, "Output format (table|json|yaml)")

	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(pkgCmd)
}

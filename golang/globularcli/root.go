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
	authAddr       string
	token          string
	caFile         string
	insecure       bool
	timeout        time.Duration
	output         string
}{
	// Bare FQDNs: resolveGRPCAddr appends :443 (Envoy mesh) at dial time.
	// Provide an explicit host:port to bypass the mesh (e.g. for bootstrap/join).
	controllerAddr: "globular.internal",
	nodeAddr:       "globular.internal",
	dnsAddr:        "globular.internal",
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
		// Priority: --ca flag → user PKI → legacy tls/ path → system CA.
		if rootCfg.caFile != "" {
			if _, err := os.Stat(rootCfg.caFile); err != nil {
				return fmt.Errorf("--ca: %w", err)
			}
		} else {
			// Not explicitly provided — try to resolve automatically.
			if caPath, err := resolveCAPath(); err == nil {
				rootCfg.caFile = caPath
			}
			// Failure is non-fatal here; individual commands will surface the error
			// when they actually attempt a TLS connection.
		}
		if rootCfg.caFile != "" {
			// Export so that service client code finds the right CA regardless
			// of install layout (InitClient → GetEtcdTLS → GetCACertificatePath).
			_ = os.Setenv("GLOBULAR_CA_CERT", rootCfg.caFile)
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
	rootCmd.PersistentFlags().StringVar(&rootCfg.authAddr, "auth", "", "Authentication service gRPC endpoint (bypasses mesh routing, e.g. for Day-0 bootstrap)")
	rootCmd.PersistentFlags().StringVar(&rootCfg.token, "token", "", "Authorization token for the control plane")
	rootCmd.PersistentFlags().BoolVar(&rootCfg.insecure, "insecure", false, "Skip TLS verification")
	rootCmd.PersistentFlags().StringVar(&rootCfg.caFile, "ca", "", "Path to CA bundle")
	rootCmd.PersistentFlags().DurationVar(&rootCfg.timeout, "timeout", rootCfg.timeout, "Request timeout")
	rootCmd.PersistentFlags().StringVar(&rootCfg.output, "output", rootCfg.output, "Output format (table|json|yaml)")

	rootCmd.AddCommand(appCmd)
	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(pkgCmd)
	rootCmd.AddCommand(servicesCmd)
	rootCmd.AddCommand(namespaceCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(objectstoreCmd)
}

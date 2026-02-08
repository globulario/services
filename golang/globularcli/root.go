package main

import (
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

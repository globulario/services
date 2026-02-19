package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	logpb "github.com/globulario/services/golang/log/logpb"
)

var (
	logsFollow bool
	logsSince  string
	logsLines  int
)

var logsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "View logs for a service",
	Long: `View logs for a Globular service.

Attempts to use the log_server RPC if available, falls back to journalctl.

Examples:
  globular logs gateway
  globular logs gateway --follow
  globular logs gateway --since 10m --lines 100
  globular logs xds --follow --since 1h
`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since duration or timestamp (e.g., 10m, 1h, 2006-01-02)")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 200, "Number of lines to show from the end")

	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Try to use log_server RPC first
	if err := tryLogServerRPC(serviceName); err == nil {
		return nil
	}

	// Fall back to journalctl
	fmt.Fprintf(os.Stderr, "Note: Using journalctl fallback (log_server unavailable)\n\n")
	return runJournalctlFallback(serviceName)
}

func tryLogServerRPC(serviceName string) error {
	// Try to connect to log_server (default port might be different, check your setup)
	// For now, assume it's on localhost:10030 or similar
	logServerAddr := getEnv("LOG_SERVER_ADDR", "localhost:10030")

	cc, err := dialGRPC(logServerAddr)
	if err != nil {
		return fmt.Errorf("connect to log_server: %w", err)
	}
	defer cc.Close()

	client := logpb.NewLogServiceClient(cc)

	// Build query - filter by service/application name
	query := fmt.Sprintf("application:%s", serviceName)

	// Add time filter if specified
	if logsSince != "" {
		query = fmt.Sprintf("%s since:%s", query, logsSince)
	}

	req := &logpb.GetLogRqst{
		Query: query,
	}

	return streamLogsRPC(client, req)
}

func streamLogsRPC(client logpb.LogServiceClient, req *logpb.GetLogRqst) error {
	ctx := context.Background()

	stream, err := client.GetLog(ctx, req)
	if err != nil {
		return fmt.Errorf("get log: %w", err)
	}

	count := 0
	maxLines := logsLines

	for {
		resp, err := stream.Recv()
		if err != nil {
			// End of stream
			if err.Error() == "EOF" {
				return nil
			}
			return fmt.Errorf("receive log: %w", err)
		}

		// Print log entries
		for _, entry := range resp.GetInfos() {
			timestamp := time.UnixMilli(entry.GetTimestampMs()).Format(time.RFC3339)
			level := entry.GetLevel().String()
			message := entry.GetMessage()

			fmt.Printf("%s [%s] %s: %s\n", timestamp, level, entry.GetApplication(), message)

			count++
			if !logsFollow && count >= maxLines {
				return nil
			}
		}
	}
}

func runJournalctlFallback(serviceName string) error {
	// Map service name to systemd unit
	unitName := mapServiceToUnit(serviceName)

	args := []string{"-u", unitName, "--no-pager"}

	if logsSince != "" {
		args = append(args, "--since", logsSince)
	}

	if logsFollow {
		args = append(args, "-f")
	} else {
		args = append(args, "-n", fmt.Sprintf("%d", logsLines))
	}

	cmd := exec.Command("journalctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func mapServiceToUnit(serviceName string) string {
	// Normalize service name to systemd unit
	serviceName = strings.ToLower(strings.TrimSpace(serviceName))

	// Common mappings
	mappings := map[string]string{
		"gateway":           "globular-gateway.service",
		"globular-gateway":  "globular-gateway.service",
		"xds":               "globular-xds.service",
		"globular-xds":      "globular-xds.service",
		"envoy":             "envoy.service",
		"etcd":              "etcd.service",
		"scylla":            "scylla.service",
		"minio":             "minio.service",
		"dns":               "globular-dns.service",
		"globular-dns":      "globular-dns.service",
		"nodeagent":         "globular-nodeagent.service",
		"globular-nodeagent": "globular-nodeagent.service",
		"node-agent":        "globular-nodeagent.service",
		"controller":        "globular-clustercontroller.service",
		"clustercontroller": "globular-clustercontroller.service",
		"globular-clustercontroller": "globular-clustercontroller.service",
	}

	if unit, ok := mappings[serviceName]; ok {
		return unit
	}

	// If not in mappings, assume it's a globular service
	if !strings.HasSuffix(serviceName, ".service") {
		serviceName = fmt.Sprintf("globular-%s.service", serviceName)
	}

	return serviceName
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

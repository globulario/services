package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ─── node logs ───────────────────────────────────────────────────────────────

var (
	nodeLogsUnit     string
	nodeLogsLines    int32
	nodeLogsPriority string
)

var nodeLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Get service logs from a node",
	Long: `Retrieve journal logs for a specific systemd unit via the Node Agent.

Examples:
  globular node logs --unit authentication --lines 100
  globular node logs --unit etcd --priority err
  globular node logs --unit postgresql --node 10.0.0.8:11000
`,
	RunE: runNodeLogs,
}

func runNodeLogs(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(rootCfg.nodeAddr)
	if err != nil {
		return fmt.Errorf("connect to node agent %s: %w", rootCfg.nodeAddr, err)
	}
	defer cc.Close()

	client := node_agentpb.NewNodeAgentServiceClient(cc)

	unit := normalizeUnit(nodeLogsUnit)
	resp, err := client.GetServiceLogs(ctxWithTimeout(), &node_agentpb.GetServiceLogsRequest{
		Unit:     unit,
		Lines:    nodeLogsLines,
		Priority: nodeLogsPriority,
	})
	if err != nil {
		return fmt.Errorf("get service logs: %w", err)
	}

	for _, line := range resp.GetLines() {
		fmt.Println(line)
	}
	return nil
}

// ─── node search-logs ────────────────────────────────────────────────────────

var (
	nodeSearchUnit     string
	nodeSearchPattern  string
	nodeSearchSince    string
	nodeSearchUntil    string
	nodeSearchPriority string
	nodeSearchLimit    int32
)

var nodeSearchLogsCmd = &cobra.Command{
	Use:   "search-logs",
	Short: "Search service logs with pattern matching",
	Long: `Search journal logs for a unit with regex pattern and time range filtering.

Examples:
  globular node search-logs --unit authentication --pattern "error|timeout"
  globular node search-logs --unit etcd --pattern "leader" --since "1 hour ago"
  globular node search-logs --unit postgresql --pattern "fatal" --priority err
`,
	RunE: runNodeSearchLogs,
}

func runNodeSearchLogs(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(rootCfg.nodeAddr)
	if err != nil {
		return fmt.Errorf("connect to node agent %s: %w", rootCfg.nodeAddr, err)
	}
	defer cc.Close()

	client := node_agentpb.NewNodeAgentServiceClient(cc)

	unit := normalizeUnit(nodeSearchUnit)
	resp, err := client.SearchServiceLogs(ctxWithTimeout(), &node_agentpb.SearchServiceLogsRequest{
		Unit:     unit,
		Pattern:  nodeSearchPattern,
		Since:    nodeSearchSince,
		Until:    nodeSearchUntil,
		Priority: nodeSearchPriority,
		Limit:    nodeSearchLimit,
	})
	if err != nil {
		return fmt.Errorf("search service logs: %w", err)
	}

	for _, line := range resp.GetLines() {
		fmt.Println(line)
	}

	if resp.GetMatchCount() > 0 {
		fmt.Printf("\n--- %d matches", resp.GetMatchCount())
		if resp.GetTruncated() {
			fmt.Printf(" (truncated)")
		}
		fmt.Printf(" ---\n")
	}
	return nil
}

// ─── node certificate-status ─────────────────────────────────────────────────

var nodeCertStatusCmd = &cobra.Command{
	Use:   "certificate-status",
	Short: "Show TLS certificate status on a node",
	Long: `Query the Node Agent for TLS certificate information including
subject, issuer, SANs, expiry dates, and chain validity.

Examples:
  globular node certificate-status
  globular node certificate-status --node 10.0.0.8:11000
`,
	RunE: runNodeCertStatus,
}

func runNodeCertStatus(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(rootCfg.nodeAddr)
	if err != nil {
		return fmt.Errorf("connect to node agent %s: %w", rootCfg.nodeAddr, err)
	}
	defer cc.Close()

	client := node_agentpb.NewNodeAgentServiceClient(cc)

	resp, err := client.GetCertificateStatus(ctxWithTimeout(), &node_agentpb.GetCertificateStatusRequest{})
	if err != nil {
		return fmt.Errorf("get certificate status: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	if sc := resp.GetServerCert(); sc != nil {
		fmt.Printf("Server Certificate:\n")
		printCertInfo(sc)
	}

	if ca := resp.GetCaCert(); ca != nil {
		fmt.Printf("\nCA Certificate:\n")
		printCertInfo(ca)
	}

	return nil
}

func printCertInfo(ci *node_agentpb.CertificateInfo) {
	fmt.Printf("  Subject:           %s\n", ci.GetSubject())
	fmt.Printf("  Issuer:            %s\n", ci.GetIssuer())
	if len(ci.GetSans()) > 0 {
		fmt.Printf("  SANs:              %s\n", strings.Join(ci.GetSans(), ", "))
	}
	fmt.Printf("  Not Before:        %s\n", ci.GetNotBefore())
	fmt.Printf("  Not After:         %s\n", ci.GetNotAfter())
	fmt.Printf("  Days Until Expiry: %d\n", ci.GetDaysUntilExpiry())
	if ci.GetFingerprint() != "" {
		fmt.Printf("  SHA256:            %s\n", ci.GetFingerprint())
	}
	if ci.GetChainValid() {
		fmt.Printf("  Chain Valid:       true\n")
	}
}

// ─── node control ────────────────────────────────────────────────────────────

var (
	nodeControlUnit   string
	nodeControlAction string
)

var nodeControlCmd = &cobra.Command{
	Use:   "control",
	Short: "Control a service on a node (start/stop/restart/status)",
	Long: `Send a control action to a systemd unit via the Node Agent.

Actions: start, stop, restart, status

Examples:
  globular node control --unit authentication --action restart
  globular node control --unit etcd --action status
  globular node control --unit postgresql --action stop --node 10.0.0.8:11000
`,
	RunE: runNodeControl,
}

func runNodeControl(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(rootCfg.nodeAddr)
	if err != nil {
		return fmt.Errorf("connect to node agent %s: %w", rootCfg.nodeAddr, err)
	}
	defer cc.Close()

	client := node_agentpb.NewNodeAgentServiceClient(cc)

	unit := normalizeUnit(nodeControlUnit)
	resp, err := client.ControlService(ctxWithTimeout(), &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: nodeControlAction,
	})
	if err != nil {
		return fmt.Errorf("control service: %w", err)
	}

	if resp.GetOk() {
		fmt.Printf("%s %s: %s (%s)\n", nodeControlAction, nodeControlUnit, resp.GetState(), resp.GetMessage())
	} else {
		fmt.Printf("%s %s: FAILED — %s\n", nodeControlAction, nodeControlUnit, resp.GetMessage())
	}
	return nil
}

// normalizeUnit adds the globular- prefix if the name looks like a bare service name.
func normalizeUnit(name string) string {
	if !strings.HasPrefix(name, "globular-") && !strings.HasPrefix(name, "scylla") && !strings.Contains(name, ".") {
		return "globular-" + name
	}
	return name
}

// ─── node uninstall-package ────────────────────────────────────────────────────
//
// SCAR-5 (placement.orphan_removal_needs_a_lawful_node_scoped_path): a lawful,
// NODE-SCOPED uninstall — invokes the Node Agent's existing supervisor-mediated
// "uninstall-package" workflow on ONE node (stop+disable, remove files, clear
// installed-state) WITHOUT touching cluster desired-state. The correct lever for a
// placement orphan; `services desired remove` would retire the service cluster-wide.

var (
	nodeUninstallPkg  string
	nodeUninstallKind string
)

var nodeUninstallCmd = &cobra.Command{
	Use:   "uninstall-package",
	Short: "Uninstall a package from ONE node (node-scoped), clearing its installed-state",
	Long: `Uninstall a package from a single node via the Node Agent's supervisor-mediated
uninstall-package workflow: stops+disables the unit, removes files, and clears
installed-state — WITHOUT touching cluster desired-state.

Use to retire a placement orphan (a package installed on a node whose profiles do
not authorize it). To retire a service across the whole cluster use
'globular services desired remove' instead. Removing an authorized package this way
is self-healing — the reconciler reinstalls it.

Examples:
  globular node uninstall-package --package torrent --node 10.0.0.8:11000
  globular node uninstall-package --package yt-dlp --kind COMMAND --node 10.0.0.8:11000
`,
	RunE: runNodeUninstall,
}

// buildUninstallPackageWorkflow builds the node-agent RunWorkflow request that drives
// the existing supervisor-mediated "uninstall-package" workflow on ONE node. It carries
// only the package identity — it never references or mutates cluster desired-state.
func buildUninstallPackageWorkflow(pkg, kind string) *node_agentpb.RunWorkflowRequest {
	if kind == "" {
		kind = "SERVICE"
	}
	return &node_agentpb.RunWorkflowRequest{
		WorkflowName: "uninstall-package",
		Inputs: map[string]string{
			"package_name": pkg,
			"kind":         strings.ToUpper(kind),
		},
	}
}

func runNodeUninstall(cmd *cobra.Command, args []string) error {
	if nodeUninstallPkg == "" {
		return fmt.Errorf("--package is required")
	}
	cc, err := dialGRPC(rootCfg.nodeAddr)
	if err != nil {
		return fmt.Errorf("connect to node agent %s: %w", rootCfg.nodeAddr, err)
	}
	defer func() { _ = cc.Close() }()

	client := node_agentpb.NewNodeAgentServiceClient(cc)
	resp, err := client.RunWorkflow(ctxWithTimeout(), buildUninstallPackageWorkflow(nodeUninstallPkg, nodeUninstallKind))
	if err != nil {
		return fmt.Errorf("uninstall-package: %w", err)
	}

	if resp.GetStatus() == "SUCCEEDED" {
		fmt.Printf("uninstalled %s: %s (%d/%d steps)\n",
			nodeUninstallPkg, resp.GetStatus(), resp.GetStepsSucceeded(), resp.GetStepsTotal())
	} else {
		fmt.Printf("uninstall %s: %s — %s\n", nodeUninstallPkg, resp.GetStatus(), resp.GetError())
	}
	return nil
}

func init() {
	nodeLogsCmd.Flags().StringVar(&nodeLogsUnit, "unit", "", "Systemd unit name (required)")
	nodeLogsCmd.Flags().Int32Var(&nodeLogsLines, "lines", 50, "Number of log lines")
	nodeLogsCmd.Flags().StringVar(&nodeLogsPriority, "priority", "", "Priority filter: emerg, alert, crit, err, warning, notice, info, debug")
	nodeLogsCmd.MarkFlagRequired("unit")

	nodeSearchLogsCmd.Flags().StringVar(&nodeSearchUnit, "unit", "", "Systemd unit name (required)")
	nodeSearchLogsCmd.Flags().StringVar(&nodeSearchPattern, "pattern", "", "Regex pattern")
	nodeSearchLogsCmd.Flags().StringVar(&nodeSearchSince, "since", "", "Start time")
	nodeSearchLogsCmd.Flags().StringVar(&nodeSearchUntil, "until", "", "End time")
	nodeSearchLogsCmd.Flags().StringVar(&nodeSearchPriority, "priority", "", "Priority filter")
	nodeSearchLogsCmd.Flags().Int32Var(&nodeSearchLimit, "limit", 100, "Max lines")
	nodeSearchLogsCmd.MarkFlagRequired("unit")

	nodeControlCmd.Flags().StringVar(&nodeControlUnit, "unit", "", "Systemd unit name (required)")
	nodeControlCmd.Flags().StringVar(&nodeControlAction, "action", "", "Action: start, stop, restart, status")
	nodeControlCmd.MarkFlagRequired("unit")
	nodeControlCmd.MarkFlagRequired("action")

	nodeUninstallCmd.Flags().StringVar(&nodeUninstallPkg, "package", "", "Package/service name to uninstall (required)")
	nodeUninstallCmd.Flags().StringVar(&nodeUninstallKind, "kind", "SERVICE", "Package kind: SERVICE, INFRASTRUCTURE, COMMAND")
	_ = nodeUninstallCmd.MarkFlagRequired("package")

	nodeCmd.AddCommand(nodeLogsCmd)
	nodeCmd.AddCommand(nodeSearchLogsCmd)
	nodeCmd.AddCommand(nodeCertStatusCmd)
	nodeCmd.AddCommand(nodeControlCmd)
	nodeCmd.AddCommand(nodeUninstallCmd)
}

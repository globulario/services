package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/yaml"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/dns/dnspb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/security"
)

const (
	defaultUpgradeTargetPath = "/usr/local/bin/globular"
	defaultUpgradeProbePort  = 80
)

var (
	clusterCmd = &cobra.Command{
		Use:   "cluster",
		Short: "Control plane helpers",
	}

	bootstrapNodeAddr string
	bootstrapDomain   string
	bootstrapBind     string
	bootstrapProfiles []string

	joinNodeOverride       string
	joinControllerOverride string
	joinToken              string

	tokenExpires string

	reqProfiles              []string
	reqMetadata              []string
	rejectReason             string
	requestsApproveRequestID string
	requestsRejectRequestID  string

	profileSet        []string
	profilePreviewSet []string

	removeNodeForce bool
	removeNodeDrain bool

	debugAgentEndpoint  string
	debugAgentOpID      string
	debugAgentWatchCtrl bool

	watchNodeID      string
	watchOpID        string
	upgradeNodeID    string
	upgradePlatform  string
	upgradeSha       string
	upgradeTarget    string
	upgradeProbePort int

	networkDomain     string
	networkProtocol   string
	networkHTTPPort   int
	networkHTTPSPort  int
	networkAcme       bool
	networkEmail      string
	networkAltDomains []string
	networkWatch      bool

	dnsBootstrapDomain   string
	dnsBootstrapIPv6     string
	dnsBootstrapIPv4     string
	dnsBootstrapWildcard bool
)

func init() {
	clusterCmd.AddCommand(
		bootstrapCmd,
		joinCmd,
		tokenCmd,
		requestsCmd,
		nodesCmd,
		upgradeCmd,
		watchCmd,
		networkCmd,
		clusterDnsCmd,
		rotateNodeTokensCmd,
		// healthCmd is added in health_cmds.go
	)

	bootstrapCmd.Flags().StringVar(&bootstrapNodeAddr, "node", "", "Node agent endpoint (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapDomain, "domain", "", "Cluster domain (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapBind, "bind", "0.0.0.0:12000", "Controller bind address")
	bootstrapCmd.Flags().StringSliceVar(&bootstrapProfiles, "profile", nil, "Profiles for the first node")

	joinCmd.Flags().StringVar(&joinNodeOverride, "node", "", "Node agent endpoint")
	joinCmd.Flags().StringVar(&joinControllerOverride, "controller", "", "Controller endpoint")
	joinCmd.Flags().StringVar(&joinToken, "join-token", "", "Join token")

	tokenCreateCmd.Flags().StringVar(&tokenExpires, "expires", "24h", "Token expiration (duration or RFC3339)")

	requestsApproveCmd.Flags().StringSliceVar(&reqProfiles, "profile", nil, "Profiles to assign")
	requestsApproveCmd.Flags().StringSliceVar(&reqMetadata, "meta", nil, "Metadata entries (k=v)")
	requestsApproveCmd.Flags().StringVar(&requestsApproveRequestID, "request-id", "", "Join request ID (overrides positional argument)")
	requestsRejectCmd.Flags().StringVar(&rejectReason, "reason", "", "Rejection reason")
	requestsRejectCmd.Flags().StringVar(&requestsRejectRequestID, "request-id", "", "Join request ID (overrides positional argument)")

	nodeProfilesCmd.Flags().StringSliceVar(&profileSet, "profile", nil, "Profiles to assign (required)")
	nodeProfilesPreviewCmd.Flags().StringSliceVar(&profilePreviewSet, "profile", nil, "Profiles to preview (required)")

	nodeRemoveCmd.Flags().BoolVar(&removeNodeForce, "force", false, "Force removal even if node is unreachable")
	nodeRemoveCmd.Flags().BoolVar(&removeNodeDrain, "drain", true, "Drain node (stop services gracefully) before removal")

	agentInventoryCmd.Flags().StringVar(&debugAgentEndpoint, "agent", "", "Node agent endpoint (required)")

	upgradeCmd.Flags().StringVar(&upgradeNodeID, "node-id", "", "Target node ID (required)")
	upgradeCmd.Flags().StringVar(&upgradePlatform, "platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), "Target platform (os/arch)")
	upgradeCmd.Flags().StringVar(&upgradeSha, "sha256", "", "Artifact sha256 (computed if omitted)")
	upgradeCmd.Flags().StringVar(&upgradeTarget, "target-path", "", "Destination path for the Globular binary")
	upgradeCmd.Flags().IntVar(&upgradeProbePort, "probe-port", defaultUpgradeProbePort, "HTTP port to call /checksum")

	networkCmd.AddCommand(networkSetCmd, networkGetCmd)
	networkSetCmd.Flags().StringVar(&networkDomain, "domain", "", "Cluster domain (required)")
	networkSetCmd.Flags().StringVar(&networkProtocol, "protocol", "https", "Network protocol (http|https)")
	networkSetCmd.Flags().IntVar(&networkHTTPPort, "http-port", 8080, "HTTP port to configure")
	networkSetCmd.Flags().IntVar(&networkHTTPSPort, "https-port", 8443, "HTTPS port to configure")
	networkSetCmd.Flags().BoolVar(&networkAcme, "acme", false, "Enable ACME certificate management")
	networkSetCmd.Flags().StringVar(&networkEmail, "email", "", "Admin email (required when --acme)")
	networkSetCmd.Flags().StringSliceVar(&networkAltDomains, "alt-domain", nil, "Add alternate domains")
	networkSetCmd.Flags().BoolVar(&networkWatch, "watch", false, "Watch controller operations after apply")

	watchCmd.Flags().StringVar(&watchNodeID, "node-id", "", "Filter by node ID")
	watchCmd.Flags().StringVar(&watchOpID, "op", "", "Filter by operation ID")

	clusterDnsCmd.AddCommand(clusterDnsBootstrapCmd)
	clusterDnsBootstrapCmd.Flags().StringVar(&dnsBootstrapDomain, "domain", "", "Cluster domain (required)")
	clusterDnsBootstrapCmd.Flags().StringVar(&dnsBootstrapIPv6, "ipv6", "", "IPv6 address for the domain")
	clusterDnsBootstrapCmd.Flags().StringVar(&dnsBootstrapIPv4, "ipv4", "", "IPv4 address for the domain")
	clusterDnsBootstrapCmd.Flags().BoolVar(&dnsBootstrapWildcard, "wildcard", false, "Also set wildcard *.<domain> records")
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap the first control-plane node",
	RunE: func(cmd *cobra.Command, args []string) error {
		if bootstrapDomain == "" {
			return errors.New("--domain is required")
		}
		nodeAddr := pick(bootstrapNodeAddr, rootCfg.nodeAddr)
		cc, err := dialGRPC(nodeAddr)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := node_agentpb.NewNodeAgentServiceClient(cc)
		req := &node_agentpb.BootstrapFirstNodeRequest{
			ClusterDomain:  bootstrapDomain,
			ControllerBind: bootstrapBind,
			Profiles:       bootstrapProfiles,
		}
		// Bootstrap is a long-running operation (starts services, registers node).
		// Use a generous timeout — 2 minutes minimum, or the user's --timeout if longer.
		bootstrapTimeout := 2 * time.Minute
		if rootCfg.timeout > bootstrapTimeout {
			bootstrapTimeout = rootCfg.timeout
		}
		bctx, bcancel := context.WithTimeout(context.Background(), bootstrapTimeout)
		defer bcancel()

		resp, err := client.BootstrapFirstNode(bctx, req)
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Have a node request to join an existing controller",
	RunE: func(cmd *cobra.Command, args []string) error {
		if joinToken == "" {
			return errors.New("--join-token is required")
		}
		nodeAddr := pick(joinNodeOverride, rootCfg.nodeAddr)
		controllerAddr := pick(joinControllerOverride, rootCfg.controllerAddr)
		cc, err := dialGRPC(nodeAddr)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := node_agentpb.NewNodeAgentServiceClient(cc)

		// Join is also a long operation — use 2 minute timeout.
		joinTimeout := 2 * time.Minute
		if rootCfg.timeout > joinTimeout {
			joinTimeout = rootCfg.timeout
		}
		jctx, jcancel := context.WithTimeout(context.Background(), joinTimeout)
		defer jcancel()

		resp, err := client.JoinCluster(jctx, &node_agentpb.JoinClusterRequest{
			ControllerEndpoint: controllerAddr,
			JoinToken:          joinToken,
		})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage controller join tokens",
}

var tokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a join token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		req := &cluster_controllerpb.CreateJoinTokenRequest{}
		if tokenExpires != "" {
			ts, err := parseExpiration(tokenExpires)
			if err != nil {
				return err
			}
			req.ExpiresAt = ts
		}
		resp, err := client.CreateJoinToken(ctxWithTimeout(), req)
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var requestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "Manage pending join requests",
}

var requestsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending join requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ListJoinRequests(ctxWithTimeout(), &cluster_controllerpb.ListJoinRequestsRequest{})
		if err != nil {
			return err
		}
		printJoinRequests(resp)
		return nil
	},
}

var requestsApproveCmd = &cobra.Command{
	Use:   "approve <request_id>",
	Short: "Approve a pending join request",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		requestID, err := resolveRequestID(args, requestsApproveRequestID)
		if err != nil {
			return err
		}
		resp, err := client.ApproveJoin(ctxWithTimeout(), &cluster_controllerpb.ApproveJoinRequest{
			RequestId: requestID,
			Profiles:  reqProfiles,
			Metadata:  parseMetadata(reqMetadata),
		})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var requestsRejectCmd = &cobra.Command{
	Use:   "reject <request_id>",
	Short: "Reject a pending join request",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		requestID, err := resolveRequestID(args, requestsRejectRequestID)
		if err != nil {
			return err
		}
		resp, err := client.RejectJoin(ctxWithTimeout(), &cluster_controllerpb.RejectJoinRequest{
			RequestId: requestID,
			Reason:    rejectReason,
		})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

func resolveRequestID(args []string, flagValue string) (string, error) {
	id := strings.TrimSpace(flagValue)
	if id == "" && len(args) > 0 {
		id = strings.TrimSpace(args[0])
	}
	if id == "" {
		return "", errors.New("request id is required")
	}
	return id, nil
}

func printJoinRequests(resp *cluster_controllerpb.ListJoinRequestsResponse) {
	if resp == nil || len(resp.GetPending()) == 0 {
		fmt.Println("no pending join requests")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REQUEST ID\tSTATUS\tHOSTNAME\tDOMAIN\tPROFILES")
	for _, jr := range resp.GetPending() {
		identity := jr.GetIdentity()
		host := identity.GetHostname()
		domain := identity.GetDomain()
		profiles := strings.Join(jr.GetProfiles(), ",")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", jr.GetRequestId(), jr.GetStatus(), host, domain, profiles)
	}
	w.Flush()
}

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Inspect cluster nodes",
}

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ListNodes(ctxWithTimeout(), &cluster_controllerpb.ListNodesRequest{})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var nodesGetCmd = &cobra.Command{
	Use:   "get <node_id>",
	Short: "Get detailed information about a specific node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := strings.TrimSpace(args[0])
		if nodeID == "" {
			return errors.New("node_id is required")
		}

		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

		// List nodes and find the specific one
		ctx, cancel := ctxWithCLITimeout(cmd.Context())
		defer cancel()

		resp, err := client.ListNodes(ctx, &cluster_controllerpb.ListNodesRequest{})
		if err != nil {
			return err
		}

		var foundNode *cluster_controllerpb.NodeRecord
		for _, node := range resp.GetNodes() {
			if node.GetNodeId() == nodeID {
				foundNode = node
				break
			}
		}

		if foundNode == nil {
			return fmt.Errorf("node %s not found", nodeID)
		}

		// Print node details
		fmt.Printf("Node ID: %s\n", foundNode.GetNodeId())
		fmt.Printf("Status: %s\n", foundNode.GetStatus())
		fmt.Printf("Agent Endpoint: %s\n", foundNode.GetAgentEndpoint())
		fmt.Printf("Profiles: %s\n", strings.Join(foundNode.GetProfiles(), ", "))

		if identity := foundNode.GetIdentity(); identity != nil {
			fmt.Printf("\nIdentity:\n")
			fmt.Printf("  Hostname: %s\n", identity.GetHostname())
			fmt.Printf("  Domain: %s\n", identity.GetDomain())
			if len(identity.GetIps()) > 0 {
				fmt.Printf("  IPs: %s\n", strings.Join(identity.GetIps(), ", "))
			}
			fmt.Printf("  OS/Arch: %s/%s\n", identity.GetOs(), identity.GetArch())
			fmt.Printf("  Agent Version: %s\n", identity.GetAgentVersion())
		}

		if lastSeen := foundNode.GetLastSeen(); lastSeen != nil {
			fmt.Printf("\nLast Seen: %s\n", lastSeen.AsTime().Format(time.RFC3339))
		}

		if metadata := foundNode.GetMetadata(); len(metadata) > 0 {
			fmt.Printf("\nMetadata:\n")
			for k, v := range metadata {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}

		// Plan display removed — workflow-native release pipeline replaces it.

		return nil
	},
}

var nodeProfilesCmd = &cobra.Command{
	Use:   "profiles set <node_id>",
	Short: "Replace a node's profiles",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(profileSet) == 0 {
			return errors.New("--profile is required")
		}
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		_, err = client.SetNodeProfiles(ctxWithTimeout(), &cluster_controllerpb.SetNodeProfilesRequest{
			NodeId:   args[0],
			Profiles: profileSet,
		})
		if err != nil {
			return err
		}
		fmt.Println("profiles intent recorded")
		return nil
	},
}

// nodeProfilesPreviewCmd shows what WOULD happen if profiles were changed, without applying.
var nodeProfilesPreviewCmd = &cobra.Command{
	Use:   "profiles preview <node_id>",
	Short: "Preview the unit and config changes that would result from a profile change",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(profilePreviewSet) == 0 {
			return errors.New("--profile is required")
		}
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.PreviewNodeProfiles(ctxWithTimeout(), &cluster_controllerpb.PreviewNodeProfilesRequest{
			NodeId:   args[0],
			Profiles: profilePreviewSet,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Node: %s\n", args[0])
		fmt.Printf("Normalized profiles: [%s]\n\n", strings.Join(resp.GetNormalizedProfiles(), ", "))

		fmt.Println("Unit diff:")
		if len(resp.GetUnitDiff()) == 0 {
			fmt.Println("  (no unit changes)")
		}
		for _, a := range resp.GetUnitDiff() {
			prefix := " "
			switch a.GetAction() {
			case "enable", "start", "restart":
				prefix = "+"
			case "stop", "disable":
				prefix = "-"
			}
			fmt.Printf("  %s %-10s %s\n", prefix, strings.ToUpper(a.GetAction()), a.GetUnitName())
		}

		fmt.Println("\nConfig changes:")
		if len(resp.GetConfigDiff()) == 0 {
			fmt.Println("  (no config files)")
		}
		for _, d := range resp.GetConfigDiff() {
			status := "unchanged"
			if d.GetChanged() {
				if d.GetOldHash() == "" {
					status = "NEW"
				} else {
					status = fmt.Sprintf("CHANGED: %s...→%s...", truncate(d.GetOldHash(), 8), truncate(d.GetNewHash(), 8))
				}
			}
			fmt.Printf("  %-55s %s\n", d.GetPath(), status)
		}

		if len(resp.GetRestartUnits()) > 0 {
			fmt.Println("\nUnits that would be restarted due to config changes:")
			for _, u := range resp.GetRestartUnits() {
				fmt.Printf("  ~ RESTART  %s\n", u)
			}
		}

		if len(resp.GetAffectedNodes()) > 0 {
			fmt.Println("\nOther nodes whose configs would change (membership impact):")
			for _, an := range resp.GetAffectedNodes() {
				fmt.Printf("  Node: %s\n", an.GetNodeId())
				for _, d := range an.GetConfigDiff() {
					if !d.GetChanged() {
						continue
					}
					status := "CHANGED"
					if d.GetOldHash() == "" {
						status = "NEW"
					}
					fmt.Printf("    %-53s %s\n", d.GetPath(), status)
				}
			}
		}

		return nil
	},
}

// truncate returns the first n bytes of s, or s if shorter.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var nodeRemoveCmd = &cobra.Command{
	Use:   "remove <node_id>",
	Short: "Remove a node from the cluster",
	Long: `Remove a node from the cluster. By default, this will attempt to drain
the node (stop services gracefully) before removal. Use --drain=false to skip draining.
Use --force to remove even if the node is unreachable.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := strings.TrimSpace(args[0])
		if nodeID == "" {
			return errors.New("node_id is required")
		}

		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

		resp, err := client.RemoveNode(ctxWithTimeout(), &cluster_controllerpb.RemoveNodeRequest{
			NodeId: nodeID,
			Force:  removeNodeForce,
			Drain:  removeNodeDrain,
		})
		if err != nil {
			return err
		}

		fmt.Printf("operation_id: %s\n", "deprecated")
		fmt.Printf("message: %s\n", resp.GetMessage())
		return nil
	},
}

var clusterDnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS configuration helpers for cluster setup",
}

var clusterDnsBootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap DNS with cluster domain and addresses",
	Long: `Bootstrap DNS configuration by:
  1. Adding the domain to managed domains
  2. Setting apex A/AAAA records
  3. Optionally setting wildcard *.<domain> records

Example:
  globularcli cluster dns bootstrap --domain globular.io --ipv6 fd12::1 --ipv4 192.168.1.10 --wildcard`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dnsBootstrapDomain == "" {
			return errors.New("--domain is required")
		}
		if dnsBootstrapIPv6 == "" && dnsBootstrapIPv4 == "" {
			return errors.New("at least one of --ipv6 or --ipv4 is required")
		}

		domain := strings.TrimSpace(dnsBootstrapDomain)

		// Connect to DNS service
		cc, err := dialGRPC(rootCfg.dnsAddr)
		if err != nil {
			return fmt.Errorf("connect to DNS service: %w", err)
		}
		defer cc.Close()

		client := dnspb.NewDnsServiceClient(cc)
		ctx := ctxWithTimeout()

		// Step 1: Add domain to managed domains
		fmt.Printf("Adding %s to managed domains...\n", domain)
		cur, err := client.GetDomains(ctx, &dnspb.GetDomainsRequest{})
		if err != nil {
			return fmt.Errorf("get domains: %w", err)
		}

		domains := append(cur.Domains, domain)
		_, err = client.SetDomains(ctx, &dnspb.SetDomainsRequest{Domains: domains})
		if err != nil {
			return fmt.Errorf("set domains: %w", err)
		}

		// Step 2: Set apex records
		if dnsBootstrapIPv4 != "" {
			fmt.Printf("Setting A record for %s -> %s\n", domain, dnsBootstrapIPv4)
			_, err = client.SetA(ctx, &dnspb.SetARequest{Domain: domain, A: dnsBootstrapIPv4, Ttl: 300})
			if err != nil {
				return fmt.Errorf("set A record: %w", err)
			}
		}

		if dnsBootstrapIPv6 != "" {
			fmt.Printf("Setting AAAA record for %s -> %s\n", domain, dnsBootstrapIPv6)
			_, err = client.SetAAAA(ctx, &dnspb.SetAAAARequest{Domain: domain, Aaaa: dnsBootstrapIPv6, Ttl: 300})
			if err != nil {
				return fmt.Errorf("set AAAA record: %w", err)
			}
		}

		// Step 3: Optionally set wildcard records
		if dnsBootstrapWildcard {
			wildcardDomain := "*." + domain

			if dnsBootstrapIPv4 != "" {
				fmt.Printf("Setting wildcard A record for %s -> %s\n", wildcardDomain, dnsBootstrapIPv4)
				_, err = client.SetA(ctx, &dnspb.SetARequest{Domain: wildcardDomain, A: dnsBootstrapIPv4, Ttl: 300})
				if err != nil {
					return fmt.Errorf("set wildcard A record: %w", err)
				}
			}

			if dnsBootstrapIPv6 != "" {
				fmt.Printf("Setting wildcard AAAA record for %s -> %s\n", wildcardDomain, dnsBootstrapIPv6)
				_, err = client.SetAAAA(ctx, &dnspb.SetAAAARequest{Domain: wildcardDomain, Aaaa: dnsBootstrapIPv6, Ttl: 300})
				if err != nil {
					return fmt.Errorf("set wildcard AAAA record: %w", err)
				}
			}
		}

		fmt.Println("DNS bootstrap complete!")
		return nil
	},
}

// healthCmd is defined in health_cmds.go
var healthCmd *cobra.Command

var agentInventoryCmd = &cobra.Command{
	Use:   "inventory",
	Short: "Fetch node inventory from the agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint := strings.TrimSpace(debugAgentEndpoint)
		if endpoint == "" {
			return errors.New("--agent is required")
		}
		cc, err := nodeClientWith(endpoint)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := node_agentpb.NewNodeAgentServiceClient(cc)
		resp, err := client.GetInventory(ctxWithTimeout(), &node_agentpb.GetInventoryRequest{})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage cluster network configuration",
}

var networkGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Display current cluster network configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

		ctx, cancel := ctxWithCLITimeout(cmd.Context())
		defer cancel()

		nodesResp, err := client.ListNodes(ctx, &cluster_controllerpb.ListNodesRequest{})
		if err != nil {
			return err
		}

		if len(nodesResp.GetNodes()) == 0 {
			fmt.Println("No nodes in cluster - network configuration not yet initialized")
			return nil
		}

		// Plan-based network display removed — use cluster network resource.
		_ = nodesResp
		fmt.Println("Network display via plans removed — use 'globular cluster network' instead")
		return nil
		// Dead code below preserved to avoid structural changes.
		var spec cluster_controllerpb.ClusterNetworkSpec
		_ = spec
		genStr := ""

		fmt.Printf("Cluster Network Configuration:\n")
		fmt.Printf("  Domain:            %s\n", spec.GetClusterDomain())
		fmt.Printf("  Protocol:          %s\n", spec.GetProtocol())
		fmt.Printf("  HTTP Port:         %d\n", spec.GetPortHttp())
		fmt.Printf("  HTTPS Port:        %d\n", spec.GetPortHttps())
		fmt.Printf("  ACME Enabled:      %t\n", spec.GetAcmeEnabled())
		if spec.GetAdminEmail() != "" {
			fmt.Printf("  Admin Email:       %s\n", spec.GetAdminEmail())
		}
		if len(spec.GetAlternateDomains()) > 0 {
			fmt.Printf("  Alternate Domains: %s\n", strings.Join(spec.GetAlternateDomains(), ", "))
		}
		if genStr != "" {
			fmt.Printf("  Generation:        %s\n", genStr)
		}

		return nil
	},
}

var networkSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update the cluster domain/protocol configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if networkDomain == "" {
			return errors.New("--domain is required")
		}
		protocol := strings.ToLower(strings.TrimSpace(networkProtocol))
		if protocol == "" {
			protocol = "https"
		}
		if protocol != "http" && protocol != "https" {
			return errors.New("--protocol must be http or https")
		}
		if networkAcme && strings.TrimSpace(networkEmail) == "" {
			return errors.New("--email is required when --acme")
		}
		spec := &cluster_controllerpb.ClusterNetworkSpec{
			ClusterDomain:    strings.TrimSpace(networkDomain),
			Protocol:         protocol,
			PortHttp:         uint32(networkHTTPPort),
			PortHttps:        uint32(networkHTTPSPort),
			AlternateDomains: normalizeAltDomains(networkAltDomains),
			AcmeEnabled:      networkAcme,
			AdminEmail:       strings.TrimSpace(networkEmail),
		}

		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

		ctx, cancel := ctxWithCLITimeout(cmd.Context())
		defer cancel()
		resp, err := client.UpdateClusterNetwork(ctx, &cluster_controllerpb.UpdateClusterNetworkRequest{
			Spec: spec,
		})
		if err != nil {
			return err
		}
		fmt.Printf("network generation: %d\n", resp.GetGeneration())
		targetGen := resp.GetGeneration()

		ctxNodes, cancelNodes := ctxWithCLITimeout(cmd.Context())
		nodesResp, err := client.ListNodes(ctxNodes, &cluster_controllerpb.ListNodesRequest{})
		cancelNodes()
		if err != nil {
			return err
		}
		if len(nodesResp.GetNodes()) == 0 {
			fmt.Println("no nodes registered")
			return nil
		}

		for _, node := range nodesResp.GetNodes() {
			if node.GetAgentEndpoint() == "" {
				fmt.Fprintf(os.Stderr, "node %s has no agent endpoint; skipping\n", node.GetNodeId())
				continue
			}
			fmt.Fprintf(os.Stderr, "node %s: plan dispatch removed — use workflow-native release pipeline\n", node.GetNodeId())
		}
		if networkWatch {
			fmt.Printf("watching convergence to generation %d (timeout %s)\n", targetGen, rootCfg.timeout)
			if err := watchNetworkConvergence(cmd.Context(), client, targetGen); err != nil {
				return err
			}
		}
		return nil
	},
}

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Low-level debug helpers",
	Long:  "Bypasses the cluster-controller; for troubleshooting only.",
}

var debugAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run debug helpers against a node agent",
	Long:  "Bypasses the cluster-controller; for troubleshooting only.",
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade <artifact>",
	Short: "Upgrade the Globular service via controller",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := strings.TrimSpace(upgradeNodeID)
		if nodeID == "" {
			return errors.New("--node-id is required")
		}
		platform := strings.TrimSpace(upgradePlatform)
		if platform == "" {
			platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
		}
		targetPath := strings.TrimSpace(upgradeTarget)
		if targetPath == "" {
			targetPath = defaultUpgradeTargetPath
		}
		data, err := ioutil.ReadFile(args[0])
		if err != nil {
			return err
		}
		sha := strings.TrimSpace(upgradeSha)
		if sha == "" {
			sum := sha256.Sum256(data)
			sha = hex.EncodeToString(sum[:])
		} else {
			sha = strings.ToLower(sha)
		}

		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.UpgradeGlobular(ctxWithTimeout(), &cluster_controllerpb.UpgradeGlobularRequest{
			NodeId:     nodeID,
			Platform:   platform,
			Artifact:   data,
			Sha256:     sha,
			TargetPath: targetPath,
			ProbePort:  uint32(upgradeProbePort),
		})
		if err != nil {
			return err
		}
		fmt.Printf("upgrade_id: %s\nterminal_state: %s\n", resp.GetUpgradeId(), resp.GetTerminalState())
		if resp.GetErrorMessage() != "" {
			fmt.Printf("error_message: %s\n", resp.GetErrorMessage())
		}
		return nil
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch controller operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if watchNodeID == "" && watchOpID == "" {
			return errors.New("--node-id or --op is required")
		}
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		req := &cluster_controllerpb.WatchOperationsRequest{
			NodeId:      watchNodeID,
			OperationId: watchOpID,
		}
		return watchControllerStream(cc, req)
	},
}

func init() {
	tokenCmd.AddCommand(tokenCreateCmd)
	requestsCmd.AddCommand(requestsListCmd, requestsApproveCmd, requestsRejectCmd)
	nodesCmd.AddCommand(nodesListCmd, nodesGetCmd, nodeProfilesCmd, nodeProfilesPreviewCmd, nodeRemoveCmd)
	debugCmd.AddCommand(debugAgentCmd)
	debugAgentCmd.AddCommand(agentInventoryCmd)
}

func controllerClient() (*grpc.ClientConn, error) {
	return dialGRPC(rootCfg.controllerAddr)
}

func nodeClient() (*grpc.ClientConn, error) {
	return nodeClientWith("")
}

func nodeClientWith(override string) (*grpc.ClientConn, error) {
	return dialGRPC(pick(override, rootCfg.nodeAddr))
}

// tokenCredentials implements grpc.PerRPCCredentials to attach the auth token
// on every RPC call (unary and streaming) without callers having to add it to
// each context manually.
type tokenCredentials struct {
	token string
}

func (t tokenCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"token": t.token}, nil
}

func (t tokenCredentials) RequireTransportSecurity() bool {
	// All connections use TLS (--insecure only skips cert verification).
	return true
}

func dialGRPC(addr string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{}
	if rootCfg.insecure {
		// "insecure" means TLS with skip-verify, NOT plain-text.
		// All Globular services require TLS — this flag only relaxes certificate validation.
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		})))
	} else if rootCfg.caFile != "" {
		// NOTE: --ca flag loads only the server CA, which breaks mTLS client cert auth.
		// Prefer the default path (no --ca flag) for full mTLS support.
		creds, err := credentials.NewClientTLSFromFile(rootCfg.caFile, "")
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// Default: load CA + client certificates for full mTLS.
		creds, err := getTLSCredentials()
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	// Centralized token injection: attach token on every RPC (unary + streaming).
	// This ensures all commands inherit auth without per-call metadata wiring.
	if rootCfg.token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(tokenCredentials{token: rootCfg.token}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func pick(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

// ctxWithTimeout returns a context that expires after rootCfg.timeout.
// Token injection is handled centrally by dialGRPC via PerRPCCredentials.
// Callers that need explicit cancellation should use ctxWithCLITimeout instead.
func ctxWithTimeout() context.Context { //nolint:govet
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	_ = cancel // caller cannot call cancel; context expires via deadline
	return ctx
}

func ctxWithCLITimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	// Token injection is handled centrally by dialGRPC via PerRPCCredentials.
	return context.WithTimeout(parent, rootCfg.timeout)
}

func parseExpiration(value string) (*timestamppb.Timestamp, error) {
	if value == "" {
		return nil, nil
	}
	if dur, err := time.ParseDuration(value); err == nil {
		return timestamppb.New(time.Now().Add(dur)), nil
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return timestamppb.New(ts), nil
	}
	return nil, fmt.Errorf("invalid expiration: %s", value)
}

func parseMetadata(items []string) map[string]string {
	meta := map[string]string{}
	for _, entry := range items {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		meta[parts[0]] = parts[1]
	}
	return meta
}

func watchControllerStream(cc *grpc.ClientConn, req *cluster_controllerpb.WatchOperationsRequest) error {
	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	ctx, cancel := ctxWithCLITimeout(context.Background())
	defer cancel()
	stream, err := client.WatchOperations(ctx, req)
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		printProto(event)
		if event.GetDone() {
			return nil
		}
	}
}

func watchControllerOperations(nodeID, operationID string) error {
	cc, err := controllerClient()
	if err != nil {
		return err
	}
	defer cc.Close()
	req := &cluster_controllerpb.WatchOperationsRequest{
		NodeId:      nodeID,
		OperationId: operationID,
	}
	return watchControllerStream(cc, req)
}

func watchAgentOperation(operationID, nodeOverride string) error {
	cc, err := nodeClientWith(nodeOverride)
	if err != nil {
		return err
	}
	defer cc.Close()
	client := node_agentpb.NewNodeAgentServiceClient(cc)
	ctx, cancel := ctxWithCLITimeout(context.Background())
	defer cancel()
	stream, err := client.WatchOperation(ctx, &node_agentpb.WatchOperationRequest{OperationId: operationID})
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		printProto(event)
		if event.GetDone() {
			return nil
		}
	}
}

func normalizeAltDomains(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(strings.ToLower(v))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func watchNetworkConvergence(ctx context.Context, client cluster_controllerpb.ClusterControllerServiceClient, targetGen uint64) error {
	deadline := time.Now().Add(rootCfg.timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("watch timed out after %s", rootCfg.timeout)
		}
		pollCtx, cancel := ctxWithCLITimeout(ctx)
		resp, err := client.ListNodes(pollCtx, &cluster_controllerpb.ListNodesRequest{})
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "poll list nodes: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if len(resp.GetNodes()) == 0 {
			return nil
		}
		allReady := true
		for _, n := range resp.GetNodes() {
			status := strings.ToLower(strings.TrimSpace(n.GetStatus()))
			lastSeen := ""
			if ts := n.GetLastSeen(); ts != nil {
				lastSeen = ts.AsTime().Format(time.RFC3339)
			}
			fmt.Printf("node %s: status=%s last_seen=%s\n", n.GetNodeId(), status, lastSeen)
			if status == "" || status == "converging" {
				allReady = false
			}
		}
		if allReady {
			fmt.Printf("network generation %d converged\n", targetGen)
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}


func printProto(msg proto.Message) {
	if msg == nil {
		return
	}
	switch rootCfg.output {
	case "json":
		out, _ := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
		fmt.Println(string(out))
	case "yaml":
		out, _ := protojson.Marshal(msg)
		if yamlOut, err := yaml.JSONToYAML(out); err == nil {
			fmt.Println(string(yamlOut))
		} else {
			fmt.Println(string(out))
		}
	default:
		out, _ := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
		fmt.Println(string(out))
	}
}

// ── rotate-node-tokens command ──────────────────────────────────────────────

var rotateNodeTokensCmd = &cobra.Command{
	Use:   "rotate-node-tokens",
	Short: "Rotate all node-agent tokens from sa to node-scoped identity",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return fmt.Errorf("controller connection: %w", err)
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// List all nodes
		nodesResp, err := client.ListNodes(ctx, &cluster_controllerpb.ListNodesRequest{})
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}

		for _, node := range nodesResp.GetNodes() {
			nodeID := node.GetNodeId()
			principal := "node_" + nodeID

			// Generate node-scoped token
			token, err := security.GenerateToken(
				365*24*60, // 1 year
				nodeID,
				principal,
				"node-agent",
				"",
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: generate token for %s: %v\n", nodeID, err)
				continue
			}

			// Connect to node-agent and push token
			endpoint := node.GetAgentEndpoint()
			if endpoint == "" {
				fmt.Fprintf(os.Stderr, "WARN: node %s has no endpoint, skipping\n", nodeID)
				continue
			}
			ncc, err := dialGRPC(endpoint)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: connect to node %s (%s): %v\n", nodeID, endpoint, err)
				continue
			}
			naClient := node_agentpb.NewNodeAgentServiceClient(ncc)
			_, err = naClient.RotateNodeToken(ctx, &node_agentpb.RotateNodeTokenRequest{
				NewToken:     token,
				NewPrincipal: principal,
			})
			ncc.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: rotate token for %s: %v\n", nodeID, err)
				continue
			}

			fmt.Printf("rotated: node=%s principal=%s\n", nodeID, principal)
		}

		fmt.Println("done")
		return nil
	},
}

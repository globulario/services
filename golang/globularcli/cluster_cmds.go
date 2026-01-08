package main

import (
	"context"
	"crypto/sha256"
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/yaml"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
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

	profileSet []string

	debugAgentEndpoint  string
	debugAgentPlanFile  string
	watchPlanFlag       bool
	debugAgentWatchPlan bool
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
)

func init() {
	rootCmd.AddCommand(clusterCmd)
	clusterCmd.AddCommand(
		bootstrapCmd,
		joinCmd,
		tokenCmd,
		requestsCmd,
		nodesCmd,
		planCmd,
		upgradeCmd,
		watchCmd,
		networkCmd,
	)

	bootstrapCmd.Flags().StringVar(&bootstrapNodeAddr, "node", "", "Node agent endpoint (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapDomain, "domain", "", "Cluster domain (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapBind, "bind", "0.0.0.0:10000", "Controller bind address")
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

	agentInventoryCmd.Flags().StringVar(&debugAgentEndpoint, "agent", "", "Node agent endpoint (required)")
	agentApplyCmd.Flags().StringVar(&debugAgentPlanFile, "plan-file", "", "Path to NodePlan JSON or YAML")
	agentApplyCmd.Flags().BoolVar(&debugAgentWatchPlan, "watch", false, "Watch operation on completion")
	agentWatchCmd.Flags().StringVar(&debugAgentOpID, "op", "", "Operation ID to watch")

	planApplyCmd.Flags().BoolVar(&watchPlanFlag, "watch", false, "Watch operation on completion")
	debugAgentApplyPlanCmd.Flags().StringVar(&debugAgentEndpoint, "agent", "", "Node agent endpoint (required)")
	debugAgentApplyPlanCmd.Flags().BoolVar(&debugAgentWatchCtrl, "watch", false, "Watch node-agent operation")
	upgradeCmd.Flags().StringVar(&upgradeNodeID, "node-id", "", "Target node ID (required)")
	upgradeCmd.Flags().StringVar(&upgradePlatform, "platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), "Target platform (os/arch)")
	upgradeCmd.Flags().StringVar(&upgradeSha, "sha256", "", "Artifact sha256 (computed if omitted)")
	upgradeCmd.Flags().StringVar(&upgradeTarget, "target-path", "", "Destination path for the Globular binary")
	upgradeCmd.Flags().IntVar(&upgradeProbePort, "probe-port", defaultUpgradeProbePort, "HTTP port to call /checksum")

	networkCmd.AddCommand(networkSetCmd)
	networkSetCmd.Flags().StringVar(&networkDomain, "domain", "", "Cluster domain (required)")
	networkSetCmd.Flags().StringVar(&networkProtocol, "protocol", "http", "Network protocol (http|https)")
	networkSetCmd.Flags().IntVar(&networkHTTPPort, "http-port", 8080, "HTTP port to configure")
	networkSetCmd.Flags().IntVar(&networkHTTPSPort, "https-port", 8443, "HTTPS port to configure")
	networkSetCmd.Flags().BoolVar(&networkAcme, "acme", false, "Enable ACME certificate management")
	networkSetCmd.Flags().StringVar(&networkEmail, "email", "", "Admin email (required when --acme)")
	networkSetCmd.Flags().StringSliceVar(&networkAltDomains, "alt-domain", nil, "Add alternate domains")
	networkSetCmd.Flags().BoolVar(&networkWatch, "watch", true, "Watch controller operations after apply")

	watchCmd.Flags().StringVar(&watchNodeID, "node-id", "", "Filter by node ID")
	watchCmd.Flags().StringVar(&watchOpID, "op", "", "Filter by operation ID")
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
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		req := &nodeagentpb.BootstrapFirstNodeRequest{
			ClusterDomain:  bootstrapDomain,
			ControllerBind: bootstrapBind,
			Profiles:       bootstrapProfiles,
		}
		resp, err := client.BootstrapFirstNode(ctxWithTimeout(), req)
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
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		resp, err := client.JoinCluster(ctxWithTimeout(), &nodeagentpb.JoinClusterRequest{
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		req := &clustercontrollerpb.CreateJoinTokenRequest{}
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ListJoinRequests(ctxWithTimeout(), &clustercontrollerpb.ListJoinRequestsRequest{})
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		requestID, err := resolveRequestID(args, requestsApproveRequestID)
		if err != nil {
			return err
		}
		resp, err := client.ApproveJoin(ctxWithTimeout(), &clustercontrollerpb.ApproveJoinRequest{
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		requestID, err := resolveRequestID(args, requestsRejectRequestID)
		if err != nil {
			return err
		}
		resp, err := client.RejectJoin(ctxWithTimeout(), &clustercontrollerpb.RejectJoinRequest{
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

func printJoinRequests(resp *clustercontrollerpb.ListJoinRequestsResponse) {
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ListNodes(ctxWithTimeout(), &clustercontrollerpb.ListNodesRequest{})
		if err != nil {
			return err
		}
		printProto(resp)
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		_, err = client.SetNodeProfiles(ctxWithTimeout(), &clustercontrollerpb.SetNodeProfilesRequest{
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
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		resp, err := client.GetInventory(ctxWithTimeout(), &nodeagentpb.GetInventoryRequest{})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var agentApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a plan directly to a node agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if debugAgentPlanFile == "" {
			return errors.New("--plan-file is required")
		}
		plan, err := loadPlan(debugAgentPlanFile)
		if err != nil {
			return err
		}
		endpoint := strings.TrimSpace(debugAgentEndpoint)
		if endpoint == "" {
			return errors.New("--agent is required")
		}
		cc, err := nodeClientWith(endpoint)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		resp, err := client.ApplyPlan(ctxWithTimeout(), &nodeagentpb.ApplyPlanRequest{Plan: plan})
		if err != nil {
			return err
		}
		fmt.Printf("operation_id: %s\n", resp.OperationId)
		if debugAgentWatchPlan {
			return watchAgentOperation(resp.OperationId, endpoint)
		}
		return nil
	},
}

var agentWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch an operation on a node agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if debugAgentOpID == "" {
			return errors.New("--op is required")
		}
		endpoint := strings.TrimSpace(debugAgentEndpoint)
		if endpoint == "" {
			return errors.New("--agent is required")
		}
		return watchAgentOperation(debugAgentOpID, endpoint)
	},
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Work with node plans",
}

var planGetCmd = &cobra.Command{
	Use:   "get <node_id>",
	Short: "Get the effective node plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.GetNodePlan(ctxWithTimeout(), &clustercontrollerpb.GetNodePlanRequest{NodeId: args[0]})
		if err != nil {
			return err
		}
		printProto(resp.Plan)
		return nil
	},
}

var planApplyCmd = &cobra.Command{
	Use:   "apply <node_id>",
	Short: "Request controller apply a node plan",
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ApplyNodePlan(ctxWithTimeout(), &clustercontrollerpb.ApplyNodePlanRequest{
			NodeId: nodeID,
		})
		if err != nil {
			return err
		}
		fmt.Printf("operation_id: %s\n", resp.GetOperationId())
		if watchPlanFlag {
			return watchControllerOperations(nodeID, resp.GetOperationId())
		}
		return nil
	},
}

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage cluster network configuration",
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
			protocol = "http"
		}
		if protocol != "http" && protocol != "https" {
			return errors.New("--protocol must be http or https")
		}
		if networkAcme && strings.TrimSpace(networkEmail) == "" {
			return errors.New("--email is required when --acme")
		}
		spec := &clustercontrollerpb.ClusterNetworkSpec{
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)

		resp, err := client.UpdateClusterNetwork(ctxWithTimeout(), &clustercontrollerpb.UpdateClusterNetworkRequest{
			Spec: spec,
		})
		if err != nil {
			return err
		}
		fmt.Printf("network generation: %d\n", resp.GetGeneration())

		nodesResp, err := client.ListNodes(ctxWithoutTimeout(), &clustercontrollerpb.ListNodesRequest{})
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
			planResp, err := client.ApplyNodePlan(ctxWithTimeout(), &clustercontrollerpb.ApplyNodePlanRequest{
				NodeId: node.GetNodeId(),
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "node %s apply failed: %v\n", node.GetNodeId(), err)
				continue
			}
			fmt.Printf("node %s apply started (op %s)\n", node.GetNodeId(), planResp.GetOperationId())
			if networkWatch {
				if err := watchControllerOperations(node.GetNodeId(), planResp.GetOperationId()); err != nil {
					fmt.Fprintf(os.Stderr, "watch op %s: %v\n", planResp.GetOperationId(), err)
				}
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

var debugAgentApplyPlanCmd = &cobra.Command{
	Use:   "apply-plan <node_id>",
	Short: "DEBUG ONLY: direct node-agent plan apply",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := strings.TrimSpace(args[0])
		if nodeID == "" {
			return errors.New("node_id is required")
		}
		agentEndpoint := strings.TrimSpace(debugAgentEndpoint)
		if agentEndpoint == "" {
			return errors.New("--agent is required")
		}
		fmt.Fprintln(os.Stderr, "WARNING: bypassing the controller; use only for debugging")

		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		planResp, err := client.GetNodePlan(ctxWithTimeout(), &clustercontrollerpb.GetNodePlanRequest{NodeId: nodeID})
		if err != nil {
			return err
		}
		plan := planResp.GetPlan()
		if plan == nil {
			return errors.New("controller returned empty plan")
		}

		nc, err := nodeClientWith(agentEndpoint)
		if err != nil {
			return err
		}
		defer nc.Close()
		nodeClient := nodeagentpb.NewNodeAgentServiceClient(nc)
		applyResp, err := nodeClient.ApplyPlan(ctxWithTimeout(), &nodeagentpb.ApplyPlanRequest{Plan: plan})
		if err != nil {
			return err
		}
		fmt.Printf("operation_id: %s\n", applyResp.GetOperationId())
		if debugAgentWatchCtrl {
			return watchAgentOperation(applyResp.GetOperationId(), agentEndpoint)
		}
		return nil
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade <artifact>",
	Short: "Upgrade the Globular service via controller plan",
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
			targetPath = os.Getenv("GLOBULAR_BINARY_PATH")
		}
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
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.UpgradeGlobular(ctxWithTimeout(), &clustercontrollerpb.UpgradeGlobularRequest{
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
		fmt.Printf("plan_id: %s\ngeneration: %d\nterminal_state: %s\n", resp.GetPlanId(), resp.GetGeneration(), resp.GetTerminalState())
		if resp.GetErrorStepId() != "" {
			fmt.Printf("error_step_id: %s\n", resp.GetErrorStepId())
		}
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
		req := &clustercontrollerpb.WatchOperationsRequest{
			NodeId:      watchNodeID,
			OperationId: watchOpID,
		}
		return watchControllerStream(cc, req)
	},
}

func init() {
	tokenCmd.AddCommand(tokenCreateCmd)
	requestsCmd.AddCommand(requestsListCmd, requestsApproveCmd, requestsRejectCmd)
	nodesCmd.AddCommand(nodesListCmd, nodeProfilesCmd)
	planCmd.AddCommand(planGetCmd, planApplyCmd)
	debugCmd.AddCommand(debugAgentCmd)
	debugAgentCmd.AddCommand(agentInventoryCmd, agentApplyCmd, agentWatchCmd, debugAgentApplyPlanCmd)
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

func dialGRPC(addr string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithBlock()}
	if rootCfg.insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if rootCfg.caFile != "" {
		creds, err := credentials.NewClientTLSFromFile(rootCfg.caFile, "")
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	return grpc.DialContext(ctx, addr, opts...)
}

func pick(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

func ctxWithTimeout() context.Context {
	ctx := context.Background()
	if rootCfg.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "token", rootCfg.token)
	}
	ctx, _ = context.WithTimeout(ctx, rootCfg.timeout)
	return ctx
}

func ctxWithoutTimeout() context.Context {
	ctx := context.Background()
	if rootCfg.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "token", rootCfg.token)
	}
	return ctx
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

func watchControllerStream(cc *grpc.ClientConn, req *clustercontrollerpb.WatchOperationsRequest) error {
	client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
	stream, err := client.WatchOperations(ctxWithoutTimeout(), req)
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
	req := &clustercontrollerpb.WatchOperationsRequest{
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
	client := nodeagentpb.NewNodeAgentServiceClient(cc)
	stream, err := client.WatchOperation(ctxWithoutTimeout(), &nodeagentpb.WatchOperationRequest{OperationId: operationID})
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

func loadPlan(path string) (*clustercontrollerpb.NodePlan, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan clustercontrollerpb.NodePlan
	if err := protojson.Unmarshal(data, &plan); err == nil {
		return &plan, nil
	}
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}
	if err := protojson.Unmarshal(jsonData, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
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

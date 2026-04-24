package main

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *NodeAgentServer) JoinCluster(ctx context.Context, req *node_agentpb.JoinClusterRequest) (*node_agentpb.JoinClusterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	token := strings.TrimSpace(req.GetJoinToken())
	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}
	if srv.joinToken != "" && token != srv.joinToken {
		return nil, status.Error(codes.PermissionDenied, "join token mismatch")
	}
	controllerEndpoint := strings.TrimSpace(req.GetControllerEndpoint())
	if controllerEndpoint == "" {
		return nil, status.Error(codes.InvalidArgument, "controller_endpoint is required")
	}
	if isNonRoutableEndpoint(controllerEndpoint) {
		return nil, status.Errorf(codes.InvalidArgument, "controller_endpoint must be routable, got %q", controllerEndpoint)
	}
	srv.controllerEndpoint = controllerEndpoint
	srv.state.ControllerEndpoint = controllerEndpoint
	if err := srv.saveState(); err != nil {
		fmt.Printf("warn: persist controller endpoint: %v\n", err)
	}

	if err := srv.ensureControllerClient(ctx); err != nil {
		return nil, status.Errorf(codes.Unavailable, "controller unavailable: %v", err)
	}

	resp, err := srv.controllerClient.RequestJoin(ctx, &cluster_controllerpb.RequestJoinRequest{
		JoinToken:    token,
		Identity:     srv.buildNodeIdentity(),
		Labels:       srv.joinRequestLabels(),
		Capabilities: buildNodeCapabilities(),
	})
	if err != nil {
		return nil, err
	}
	srv.joinRequestID = resp.GetRequestId()
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = ""
	srv.nodeID = ""
	if err := srv.saveState(); err != nil {
		fmt.Printf("warn: persist join request: %v\n", err)
	}

	srv.startJoinApprovalWatcher(context.Background(), srv.joinRequestID)

	return &node_agentpb.JoinClusterResponse{
		RequestId: resp.GetRequestId(),
		Status:    resp.GetStatus(),
		Message:   resp.GetMessage(),
	}, nil
}

func (srv *NodeAgentServer) GetInventory(ctx context.Context, _ *node_agentpb.GetInventoryRequest) (*node_agentpb.GetInventoryResponse, error) {
	installed, _, _ := ComputeInstalledServices(ctx)
	components := make([]*node_agentpb.InstalledComponent, 0, len(installed))
	for _, info := range installed {
		if info.ServiceName == "" {
			continue
		}
		components = append(components, &node_agentpb.InstalledComponent{
			Name:      canonicalServiceName(info.ServiceName),
			Version:   info.Version,
			Installed: true,
		})
	}

	resp := &node_agentpb.GetInventoryResponse{
		Inventory: &node_agentpb.Inventory{
			Identity:   srv.buildNodeIdentity(),
			UnixTime:   timestamppb.Now(),
			Components: components,
			Units:      detectUnits(ctx),
		},
	}
	return resp, nil
}

// ── RPCs ────────────────────────────────────────────────────────────────

func (srv *NodeAgentServer) GetServiceLogs(ctx context.Context, req *node_agentpb.GetServiceLogsRequest) (*node_agentpb.GetServiceLogsResponse, error) {
	unit := strings.TrimSpace(req.GetUnit())
	if unit == "" {
		return nil, status.Error(codes.InvalidArgument, "unit is required")
	}
	if !strings.HasPrefix(unit, "globular-") {
		return nil, status.Error(codes.InvalidArgument, "unit must start with 'globular-'")
	}

	lines := int(req.GetLines())
	if lines <= 0 {
		lines = 50
	}
	if lines > 200 {
		lines = 200
	}

	priority := strings.TrimSpace(req.GetPriority())
	output, err := supervisor.ReadJournalctl(ctx, unit, lines, priority)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "journalctl: %v", err)
	}

	logLines := strings.Split(output, "\n")
	if len(logLines) == 1 && logLines[0] == "" {
		logLines = nil
	}

	return &node_agentpb.GetServiceLogsResponse{
		Unit:      unit,
		LineCount: int32(len(logLines)),
		Lines:     logLines,
	}, nil
}

func (srv *NodeAgentServer) GetCertificateStatus(ctx context.Context, _ *node_agentpb.GetCertificateStatusRequest) (*node_agentpb.GetCertificateStatusResponse, error) {
	resp := &node_agentpb.GetCertificateStatusResponse{}

	serverCertPath := config.GetLocalServerCertificatePath()
	if serverCertPath != "" {
		resp.ServerCert = parseCertInfo(serverCertPath)
	}

	caPath := config.GetLocalCACertificate()
	if caPath != "" {
		resp.CaCert = parseCertInfo(caPath)
	}

	return resp, nil
}

func parseCertInfo(certPath string) *node_agentpb.CertificateInfo {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return &node_agentpb.CertificateInfo{
			Subject: fmt.Sprintf("error reading cert: %v", err),
		}
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return &node_agentpb.CertificateInfo{
			Subject: "error: no PEM block found",
		}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return &node_agentpb.CertificateInfo{
			Subject: fmt.Sprintf("error parsing cert: %v", err),
		}
	}

	sans := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses))
	sans = append(sans, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}

	daysUntilExpiry := int32(time.Until(cert.NotAfter).Hours() / 24)

	fingerprint := sha256.Sum256(cert.Raw)
	fpHex := fmt.Sprintf("%x", fingerprint)

	return &node_agentpb.CertificateInfo{
		Subject:         cert.Subject.CommonName,
		Issuer:          cert.Issuer.CommonName,
		Sans:            sans,
		NotBefore:       cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:        cert.NotAfter.UTC().Format(time.RFC3339),
		DaysUntilExpiry: daysUntilExpiry,
		ChainValid:      time.Now().Before(cert.NotAfter) && time.Now().After(cert.NotBefore),
		Fingerprint:     fpHex,
	}
}

// GetSubsystemHealth returns the health state of all registered background
// subsystems (goroutines) in this node agent process.
func (srv *NodeAgentServer) GetSubsystemHealth(_ context.Context, _ *node_agentpb.GetSubsystemHealthRequest) (*node_agentpb.GetSubsystemHealthResponse, error) {
	entries := globular_service.SubsystemSnapshot()
	resp := &node_agentpb.GetSubsystemHealthResponse{
		Subsystems: make([]*node_agentpb.SubsystemHealth, 0, len(entries)),
		Overall:    toProtoSubsystemState(globular_service.SubsystemOverallState()),
	}
	for _, e := range entries {
		sh := &node_agentpb.SubsystemHealth{
			Name:       e.Name,
			State:      toProtoSubsystemState(e.State),
			LastError:  e.LastError,
			ErrorCount: e.ErrorCount,
			Metadata:   e.Metadata,
		}
		if !e.LastTick.IsZero() {
			sh.LastTick = timestamppb.New(e.LastTick)
		}
		resp.Subsystems = append(resp.Subsystems, sh)
	}
	return resp, nil
}

func toProtoSubsystemState(s globular_service.SubsystemState) node_agentpb.SubsystemState {
	switch s {
	case globular_service.SubsystemHealthy:
		return node_agentpb.SubsystemState_SUBSYSTEM_STATE_HEALTHY
	case globular_service.SubsystemDegraded:
		return node_agentpb.SubsystemState_SUBSYSTEM_STATE_DEGRADED
	case globular_service.SubsystemFailed:
		return node_agentpb.SubsystemState_SUBSYSTEM_STATE_FAILED
	case globular_service.SubsystemStarting:
		return node_agentpb.SubsystemState_SUBSYSTEM_STATE_STARTING
	case globular_service.SubsystemStopped:
		return node_agentpb.SubsystemState_SUBSYSTEM_STATE_STOPPED
	default:
		return node_agentpb.SubsystemState_SUBSYSTEM_STATE_UNSPECIFIED
	}
}

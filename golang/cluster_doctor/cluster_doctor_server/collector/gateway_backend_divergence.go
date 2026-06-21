// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.snapshot
// @awareness file_role=degraded_mode_gateway_vs_backend_reachability_probe
// @awareness implements=globular.platform:intent.doctor.findings_are_operator_language
// @awareness risk=medium
package collector

// Degraded-mode gateway/backend divergence probe (PR-15).
//
// A cluster-doctor that can only reach services through the same gateway that is
// failing is useless exactly when it is needed. This probe compares two paths
// for a service: the Envoy gateway path and the direct backend port. If the
// gateway path answers with an HTML/non-gRPC content-type while the backend is
// healthy, the route — not the service — is broken. That distinction is the
// whole point of degraded-mode diagnosis: "ai_memory backend healthy; gateway
// route broken", not "ai_memory is down".
//
// Reflection does not normally route through the gateway, so a plain
// unavailable on the gateway path is inconclusive and is NOT treated as a
// divergence — only the distinctive content-type signal is.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// gatewayDivergenceServices is the set of services probed for gateway/backend
// divergence. Intentionally small in PR-15: ai_memory is the service whose
// gateway route was observed answering text/html while the backend was healthy.
var gatewayDivergenceServices = []string{"ai_memory.AiMemoryService"}

// probeResult is the outcome of one reflection probe against one endpoint.
type probeResult struct {
	reachable   bool
	html        bool
	contentType string
	err         error
}

// reflectionProber dials an endpoint and asks for the service descriptor via
// gRPC server reflection. Success => the endpoint speaks gRPC for that service.
type reflectionProber struct{}

func (reflectionProber) probe(ctx context.Context, endpoint, service string) probeResult {
	if endpoint == "" {
		return probeResult{err: errors.New("endpoint unresolved")}
	}
	opts, err := globular.InternalDialOptions()
	if err != nil {
		return probeResult{err: fmt.Errorf("dial options: %w", err)}
	}
	pctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(pctx, endpoint, opts...)
	if err != nil {
		return classifyProbeErr(err)
	}
	defer func() { _ = conn.Close() }()

	ref := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
	stream, err := ref.ServerReflectionInfo(pctx)
	if err != nil {
		return classifyProbeErr(err)
	}
	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: service,
		},
	}); err != nil {
		return classifyProbeErr(err)
	}
	resp, err := stream.Recv()
	_ = stream.CloseSend()
	if err != nil {
		return classifyProbeErr(err)
	}
	if resp.GetFileDescriptorResponse() != nil {
		return probeResult{reachable: true}
	}
	if e := resp.GetErrorResponse(); e != nil {
		return probeResult{err: fmt.Errorf("reflection error: %s", e.GetErrorMessage())}
	}
	return probeResult{err: errors.New("unexpected reflection response")}
}

// classifyProbeErr detects the distinctive misrouting signal: a gRPC client
// that received an HTML (web) response instead of a gRPC response. The grpc-go
// transport surfaces this as an "unexpected content-type" error.
func classifyProbeErr(err error) probeResult {
	if err == nil {
		return probeResult{}
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "content-type") && (strings.Contains(s, "text/html") || strings.Contains(s, "html")) {
		ct := "text/html"
		return probeResult{html: true, contentType: ct, err: err}
	}
	return probeResult{err: err}
}

// classifyGatewayBackend folds the two probe results into a snapshot record.
// Pure — unit-tested without a live cluster.
func classifyGatewayBackend(service, gwEndpoint, beEndpoint string, gw, be probeResult, nowUnix int64) GatewayBackendProbe {
	p := GatewayBackendProbe{
		Service:            service,
		GatewayEndpoint:    gwEndpoint,
		BackendEndpoint:    beEndpoint,
		GatewayReachable:   gw.reachable,
		GatewayHTML:        gw.html,
		GatewayContentType: gw.contentType,
		BackendChecked:     beEndpoint != "",
		BackendReachable:   be.reachable,
		ObservedAtUnix:     nowUnix,
	}
	if gw.err != nil {
		p.GatewayErr = gw.err.Error()
	}
	if be.err != nil {
		p.BackendErr = be.err.Error()
	}
	return p
}

// fetchGatewayBackendDivergence probes each tracked service on both paths and
// records the comparison. Best-effort: endpoint-resolution and probe failures
// fold into the snapshot as data, never abort the sweep.
func (c *Collector) fetchGatewayBackendDivergence(ctx context.Context, snap *Snapshot) {
	gwEndpoint, gerr := config.GetMeshAddress()
	if gerr != nil {
		snap.addError("gateway", "GetMeshAddress", gerr)
	}
	prober := reflectionProber{}
	now := time.Now().Unix()
	for _, svc := range gatewayDivergenceServices {
		beEndpoint := resolveBackendEndpoint(svc)

		gwRes := probeResult{err: errors.New("gateway endpoint unresolved")}
		if gwEndpoint != "" {
			gwRes = prober.probe(ctx, gwEndpoint, svc)
		}
		beRes := probeResult{err: errors.New("backend endpoint unresolved")}
		if beEndpoint != "" {
			beRes = prober.probe(ctx, beEndpoint, svc)
		}
		snap.GatewayBackendProbes = append(snap.GatewayBackendProbes,
			classifyGatewayBackend(svc, gwEndpoint, beEndpoint, gwRes, beRes, now))
	}
	if len(snap.GatewayBackendProbes) > 0 {
		snap.addSource("doctor.gateway_backend_divergence")
	}
}

// resolveBackendEndpoint returns the direct gRPC host:port for a service from
// its etcd config (Address:Port). Empty when unresolved.
func resolveBackendEndpoint(service string) string {
	all, err := config.GetServicesConfigurations()
	if err != nil {
		return ""
	}
	for _, svc := range all {
		name, _ := svc["Name"].(string)
		if !strings.EqualFold(name, service) {
			continue
		}
		port, _ := svc["Port"].(float64)
		addr, _ := svc["Address"].(string)
		if addr == "" {
			addr = config.GetRoutableIPv4()
		}
		if host, _, err := net.SplitHostPort(addr); err == nil && host != "" {
			addr = host
		}
		if port > 0 {
			return fmt.Sprintf("%s:%d", addr, int(port))
		}
	}
	return ""
}

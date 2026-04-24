package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/miekg/dns"
)

func TestRunConvergenceChecksSuccess(t *testing.T) {
	srv, err := newIPv4Server(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Skipf("skipping, cannot listen: %v", err)
	}
	defer srv.Close()

	oldMinioURL := minioHealthURLForSpec
	oldGatewayURL := gatewayHealthURLForSpec
	oldDNSAddr := dnsProbeAddr
	oldTCPAddrs := tcpProbeAddrs
	oldEnvoyUnit := envoyUnitActive
	t.Cleanup(func() {
		minioHealthURLForSpec = oldMinioURL
		gatewayHealthURLForSpec = oldGatewayURL
		dnsProbeAddr = oldDNSAddr
		tcpProbeAddrs = oldTCPAddrs
		envoyUnitActive = oldEnvoyUnit
	})
	minioHealthURLForSpec = func(spec *cluster_controllerpb.ClusterNetworkSpec, nodeIP string) string { return srv.URL }
	gatewayHealthURLForSpec = func(spec *cluster_controllerpb.ClusterNetworkSpec, nodeIP string) string { return srv.URL }
	dnsProbeAddr = func() string { return "127.0.0.1:53" }
	tcpProbeAddrs = func() map[string]string { return map[string]string{} }
	envoyUnitActive = func() error { return nil }

	origLookup := dnsLookupHost
	dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
		return []string{"127.0.0.1"}, nil
	}
	defer func() { dnsLookupHost = origLookup }()

	spec := &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		PortHttps:     443,
		PortHttp:      80,
	}
	if err := runConvergenceChecks(context.Background(), spec); err != nil {
		t.Fatalf("runConvergenceChecks: %v", err)
	}
}

func TestRunConvergenceChecksDNSFailure(t *testing.T) {
	oldMinioURL := minioHealthURLForSpec
	oldGatewayURL := gatewayHealthURLForSpec
	oldTCPAddrs := tcpProbeAddrs
	oldEnvoyUnit := envoyUnitActive
	t.Cleanup(func() {
		minioHealthURLForSpec = oldMinioURL
		gatewayHealthURLForSpec = oldGatewayURL
		tcpProbeAddrs = oldTCPAddrs
		envoyUnitActive = oldEnvoyUnit
	})
	minioHealthURLForSpec = func(spec *cluster_controllerpb.ClusterNetworkSpec, nodeIP string) string { return "http://127.0.0.1:0" }
	gatewayHealthURLForSpec = func(spec *cluster_controllerpb.ClusterNetworkSpec, nodeIP string) string { return "http://127.0.0.1:0" }
	tcpProbeAddrs = func() map[string]string {
		return map[string]string{
			"etcd":      "127.0.0.1:1",
			"minio-tcp": "127.0.0.1:1",
			"scylla":    "127.0.0.1:1",
		}
	}
	envoyUnitActive = func() error { return context.DeadlineExceeded }

	origLookup := dnsLookupHost
	dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
		return nil, context.DeadlineExceeded
	}
	defer func() { dnsLookupHost = origLookup }()

	spec := &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		PortHttps:     443,
		PortHttp:      80,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runConvergenceChecks(ctx, spec); err == nil {
		t.Fatalf("expected failure, got nil")
	}
}

func TestDNSUDPCheckPasses(t *testing.T) {
	// DNS server answering gateway.example.com A 127.0.0.1
	udpLn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("udp listen not permitted: %v", err)
	}
	defer udpLn.Close()
	dnsServer := &dns.Server{PacketConn: udpLn, Net: "udp"}
	dns.HandleFunc("gateway.example.com.", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		rr, _ := dns.NewRR("gateway.example.com. 60 IN A 127.0.0.1")
		m.Answer = append(m.Answer, rr)
		_ = w.WriteMsg(m)
	})
	go dnsServer.ActivateAndServe()
	defer dnsServer.Shutdown()

	oldDNSAddr := dnsProbeAddr
	oldTCPAddrs := tcpProbeAddrs
	oldEnvoyUnit := envoyUnitActive
	t.Cleanup(func() {
		dnsProbeAddr = oldDNSAddr
		tcpProbeAddrs = oldTCPAddrs
		envoyUnitActive = oldEnvoyUnit
	})
	dnsProbeAddr = func() string { return udpLn.LocalAddr().String() }
	tcpProbeAddrs = func() map[string]string { return map[string]string{} }
	envoyUnitActive = func() error { return nil }

	spec := &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "https", PortHttps: 443, PortHttp: 80}
	if err := runSupplementalChecks(context.Background(), spec); err != nil {
		t.Fatalf("dns supplemental check failed: %v", err)
	}
}

func TestEtcdPortCheckFails(t *testing.T) {
	oldTCPAddrs := tcpProbeAddrs
	oldEnvoyUnit := envoyUnitActive
	t.Cleanup(func() {
		tcpProbeAddrs = oldTCPAddrs
		envoyUnitActive = oldEnvoyUnit
	})
	tcpProbeAddrs = func() map[string]string {
		return map[string]string{
			"etcd":      "127.0.0.1:9",
			"minio-tcp": "127.0.0.1:0",
			"scylla":    "127.0.0.1:0",
		}
	}
	envoyUnitActive = func() error { return nil }
	origLookup := dnsLookupHost
	dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
		return []string{"127.0.0.1"}, nil
	}
	defer func() { dnsLookupHost = origLookup }()

	spec := &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80}
	err := runSupplementalChecks(context.Background(), spec)
	if err == nil || !strings.Contains(err.Error(), "etcd") {
		t.Fatalf("expected etcd dial error, got %v", err)
	}
}

func TestScyllaPortCheckPasses(t *testing.T) {
	oldTCPAddrs := tcpProbeAddrs
	oldEnvoyUnit := envoyUnitActive
	t.Cleanup(func() {
		tcpProbeAddrs = oldTCPAddrs
		envoyUnitActive = oldEnvoyUnit
	})
	tcpProbeAddrs = func() map[string]string { return map[string]string{} }
	envoyUnitActive = func() error { return nil }
	origLookup := dnsLookupHost
	dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
		return []string{"127.0.0.1"}, nil
	}
	defer func() { dnsLookupHost = origLookup }()
	spec := &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80}
	if err := runSupplementalChecks(context.Background(), spec); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func newIPv4Server(h http.Handler) (*httptest.Server, error) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := httptest.NewUnstartedServer(h)
	s.Listener = ln
	s.Start()
	return s, nil
}

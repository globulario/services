package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestRunConvergenceChecksSuccess(t *testing.T) {
	// HTTP endpoints
	srv, err := newIPv4Server(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		t.Skipf("skipping, cannot listen: %v", err)
	}
	defer srv.Close()

	// TCP listeners for supplemental checks
	etcdLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping, cannot listen: %v", err)
	}
	defer etcdLn.Close()
	minioLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping, cannot listen: %v", err)
	}
	defer minioLn.Close()
	scyllaLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping, cannot listen: %v", err)
	}
	defer scyllaLn.Close()

	t.Setenv("GLOBULAR_HEALTH_MINIO_URL", srv.URL)
	t.Setenv("GLOBULAR_HEALTH_ENVOY_URL", srv.URL)
	t.Setenv("GLOBULAR_HEALTH_GATEWAY_URL", srv.URL)
	t.Setenv("GLOBULAR_ETCD_ADDR", etcdLn.Addr().String())
	t.Setenv("GLOBULAR_MINIO_ADDR", minioLn.Addr().String())
	t.Setenv("GLOBULAR_SCYLLA_ADDR", scyllaLn.Addr().String())

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
	origLookup := dnsLookupHost
	dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
		return nil, context.DeadlineExceeded
	}
	defer func() { dnsLookupHost = origLookup }()

	// Point HTTP checks at an unreachable address. Use a short-lived context so
	// the RunChecks retry loop exits quickly instead of waiting 30s.
	t.Setenv("GLOBULAR_HEALTH_MINIO_URL", "http://127.0.0.1:0")
	t.Setenv("GLOBULAR_HEALTH_ENVOY_URL", "http://127.0.0.1:0")
	t.Setenv("GLOBULAR_HEALTH_GATEWAY_URL", "http://127.0.0.1:0")
	// Supplemental TCP probes: point to closed ports so they fail fast too.
	t.Setenv("GLOBULAR_ETCD_ADDR", "127.0.0.1:1")
	t.Setenv("GLOBULAR_MINIO_ADDR", "127.0.0.1:1")
	t.Setenv("GLOBULAR_SCYLLA_ADDR", "127.0.0.1:1")

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

	t.Setenv("GLOBULAR_DNS_UDP_ADDR", udpLn.LocalAddr().String())
	// TCP listeners to satisfy other checks
	etcdLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer etcdLn.Close()
	minioLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer minioLn.Close()
	scyllaLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer scyllaLn.Close()
	t.Setenv("GLOBULAR_ETCD_ADDR", etcdLn.Addr().String())
	t.Setenv("GLOBULAR_MINIO_ADDR", minioLn.Addr().String())
	t.Setenv("GLOBULAR_SCYLLA_ADDR", scyllaLn.Addr().String())
	t.Setenv("GLOBULAR_HEALTH_MINIO_URL", "http://127.0.0.1:0")   // skipped due to closed
	t.Setenv("GLOBULAR_HEALTH_ENVOY_URL", "http://127.0.0.1:0")   // skipped due to closed
	t.Setenv("GLOBULAR_HEALTH_GATEWAY_URL", "http://127.0.0.1:0") // skipped due to closed

	spec := &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "https", PortHttps: 443, PortHttp: 80}
	if err := runSupplementalChecks(context.Background(), spec); err != nil {
		t.Fatalf("dns supplemental check failed: %v", err)
	}
}

func TestEtcdPortCheckFails(t *testing.T) {
	t.Setenv("GLOBULAR_ETCD_ADDR", "127.0.0.1:9") // closed port
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
	// Three local listeners stand in for etcd, minio, and scylla.
	listeners := make([]net.Listener, 3)
	for i := range listeners {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("tcp listen not permitted: %v", err)
		}
		defer ln.Close()
		listeners[i] = ln
	}
	t.Setenv("GLOBULAR_ETCD_ADDR", listeners[0].Addr().String())
	t.Setenv("GLOBULAR_MINIO_ADDR", listeners[1].Addr().String())
	t.Setenv("GLOBULAR_SCYLLA_ADDR", listeners[2].Addr().String())
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

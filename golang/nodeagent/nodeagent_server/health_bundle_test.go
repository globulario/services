package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/miekg/dns"
	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
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

	spec := &clustercontrollerpb.ClusterNetworkSpec{
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

	// Avoid HTTP calls to real endpoints by pointing to unreachable but RunChecks should still attempt and fail due DNS
	t.Setenv("GLOBULAR_HEALTH_MINIO_URL", "http://127.0.0.1:0")
	t.Setenv("GLOBULAR_HEALTH_ENVOY_URL", "http://127.0.0.1:0")
	t.Setenv("GLOBULAR_HEALTH_GATEWAY_URL", "http://127.0.0.1:0")

	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		PortHttps:     443,
		PortHttp:      80,
	}
	if err := runConvergenceChecks(context.Background(), spec); err == nil {
		t.Fatalf("expected DNS failure, got nil")
	} else {
		_ = err // expected
	}
	// reset env
	os.Unsetenv("GLOBULAR_HEALTH_MINIO_URL")
	os.Unsetenv("GLOBULAR_HEALTH_ENVOY_URL")
	os.Unsetenv("GLOBULAR_HEALTH_GATEWAY_URL")
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

	spec := &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "https", PortHttps: 443, PortHttp: 80}
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

	spec := &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80}
	err := runSupplementalChecks(context.Background(), spec)
	if err == nil || !strings.Contains(err.Error(), "etcd") {
		t.Fatalf("expected etcd dial error, got %v", err)
	}
}

func TestScyllaPortCheckPasses(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listen not permitted: %v", err)
	}
	defer ln.Close()
	t.Setenv("GLOBULAR_SCYLLA_ADDR", ln.Addr().String())
	origLookup := dnsLookupHost
	dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
		return []string{"127.0.0.1"}, nil
	}
	defer func() { dnsLookupHost = origLookup }()
	spec := &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "http", PortHttp: 80}
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

package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"
)

// ProbeTLS performs a raw TLS handshake to the given address using the
// cluster CA. This surfaces the real x509 error (expired cert, SAN
// mismatch, wrong CA, etc.) instead of the generic "context deadline
// exceeded" that grpc.WithBlock() returns.
//
// Call this before any grpc.DialContext with grpc.WithBlock() to
// ensure TLS errors are explicit.
func ProbeTLS(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = "443"
	}
	tlsCfg := &tls.Config{
		ServerName: host,
	}
	caFile := GetTLSFile("", "", "ca.crt")
	if caFile != "" {
		data, err := os.ReadFile(caFile)
		if err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(data) {
				tlsCfg.RootCAs = pool
			}
		}
	}
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port), tlsCfg)
	if err != nil {
		return fmt.Errorf("TLS probe to %s (ServerName=%s): %w", addr, host, err)
	}
	conn.Close()
	return nil
}

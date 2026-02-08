package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"

	"github.com/globulario/services/golang/config"
)

// getTLSCredentials creates gRPC transport credentials with TLS for CLI connections.
// It loads the CA certificate to verify server certificates.
// Client certificates are not used - services validate clients via other means (tokens, etc).
func getTLSCredentials() (credentials.TransportCredentials, error) {
	// Try multiple CA certificate locations
	caPaths := []string{
		config.GetLocalCACertificate(), // Try config-based lookup first
		"/var/lib/globular/pki/ca.pem",
		"/var/lib/globular/pki/ca.crt",
		"/var/lib/globular/config/tls/ca.pem",
		"/var/lib/globular/config/tls/ca.crt",
	}

	var caCert []byte
	var err error
	var caPath string

	for _, path := range caPaths {
		if path == "" {
			continue
		}
		caCert, err = os.ReadFile(path)
		if err == nil {
			caPath = path
			break
		}
	}

	if caPath == "" || caCert == nil {
		return nil, fmt.Errorf("CA certificate not found (tried: /var/lib/globular/pki/ca.pem, /var/lib/globular/config/tls/ca.pem)")
	}

	// Create cert pool with CA
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", caPath)
	}

	// Create TLS config with CA verification only (no client certificates)
	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
		// ServerName is set by gRPC based on the target address
	}

	return credentials.NewTLS(tlsConfig), nil
}

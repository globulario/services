package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"

	"github.com/globulario/services/golang/config"
)

// getTLSCredentials creates gRPC transport credentials with mTLS for CLI connections.
// It loads the CA certificate to verify server certificates and client certificates
// for mutual TLS authentication (services require client certificates).
func getTLSCredentials() (credentials.TransportCredentials, error) {
	// Get domain from config
	domain, err := config.GetDomain()
	if err != nil || domain == "" {
		domain = "localhost"
	}

	// Try multiple CA certificate locations
	caPaths := []string{
		config.GetLocalCACertificate(), // Try config-based lookup first
		fmt.Sprintf("%s/.config/globular/tls/%s/ca.crt", os.Getenv("HOME"), domain),
		"/var/lib/globular/pki/ca.pem",
		"/var/lib/globular/pki/ca.crt",
		"/var/lib/globular/config/tls/ca.pem",
		"/var/lib/globular/config/tls/ca.crt",
	}

	var caCert []byte
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
		return nil, fmt.Errorf("CA certificate not found (tried: ~/.config/globular/tls/%s/ca.crt, /var/lib/globular/pki/ca.pem)", domain)
	}

	// Create cert pool with CA
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", caPath)
	}

	// Load client certificate for mTLS
	homeDir := os.Getenv("HOME")
	clientCertPaths := []struct {
		cert string
		key  string
	}{
		{
			cert: fmt.Sprintf("%s/.config/globular/tls/%s/client.crt", homeDir, domain),
			key:  fmt.Sprintf("%s/.config/globular/tls/%s/client.key", homeDir, domain),
		},
		{
			cert: "/var/lib/globular/tls/etcd/client.crt",
			key:  "/var/lib/globular/tls/etcd/client.pem",
		},
	}

	var clientCert tls.Certificate
	var certLoaded bool

	for _, paths := range clientCertPaths {
		clientCert, err = tls.LoadX509KeyPair(paths.cert, paths.key)
		if err == nil {
			certLoaded = true
			break
		}
	}

	if !certLoaded {
		return nil, fmt.Errorf("client certificate not found (tried: ~/.config/globular/tls/%s/client.{crt,key})\n"+
			"Generate certificates with: cd ~/Documents/github.com/globulario/globular-installer && ./scripts/generate-user-client-cert.sh", domain)
	}

	// Create TLS config with CA verification and client certificate
	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
		MinVersion:   tls.VersionTLS12,
		// ServerName is set by gRPC based on the target address
	}

	return credentials.NewTLS(tlsConfig), nil
}

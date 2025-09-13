package pki

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
)

// loadPair loads a certificate and private key from PEM files and parses them
// into a tls.Certificate. It also returns the raw PEM bytes for cert and key.
func loadPair(certFile, keyFile string) (tls.Certificate, []byte, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read cert %s: %w", certFile, err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read key %s: %w", keyFile, err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("parse key pair: %w", err)
	}

	return cert, certPEM, nil
}

// ValidateCertPair checks that certFile and keyFile form a valid pair and that
func (m *FileManager) ValidateCertPair(certFile, keyFile string, requireEKUs []int, requireDNS []string, requireIPs []string) error {
	cert, priv, err := loadPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load pair: %w", err)
	}
	_ = priv // we just need to ensure it parses and matches; ParseCertificate already did that in loadPair

	// EKUs (as ints to keep interface stable with your callers)
	if len(requireEKUs) > 0 {
		need := map[x509.ExtKeyUsage]bool{}
		for _, v := range requireEKUs {
			need[x509.ExtKeyUsage(v)] = true
		}
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("parse leaf certificate: %w", err)
		}
		ok := false
		for _, eku := range leaf.ExtKeyUsage {
			if need[eku] {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("certificate missing required EKU(s)")
		}
	}

	// DNS SANs
	if len(requireDNS) > 0 {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("parse leaf certificate: %w", err)
		}
		has := map[string]bool{}
		for _, d := range leaf.DNSNames {
			has[strings.ToLower(strings.TrimSpace(d))] = true
		}
		for _, want := range requireDNS {
			if !has[strings.ToLower(strings.TrimSpace(want))] {
				return fmt.Errorf("certificate missing DNS SAN %q", want)
			}
		}
	}

	// IP SANs (requireIPs as []string)
	if len(requireIPs) > 0 {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("parse leaf certificate: %w", err)
		}
		has := map[string]bool{}
		for _, ip := range leaf.IPAddresses {
			has[ip.String()] = true
		}
		for _, s := range requireIPs {
			ip := net.ParseIP(strings.TrimSpace(s))
			if ip == nil {
				return fmt.Errorf("invalid required IP %q", s)
			}
			if !has[ip.String()] {
				return fmt.Errorf("certificate missing IP SAN %q", ip.String())
			}
		}
	}

	return nil
}

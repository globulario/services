package pki

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type FileManager struct {
	ACME        ACMEConfig
	LocalCA     LocalCAConfig
	Logger      *slog.Logger
	Storage     FileStorage
	TokenSource func(ctx context.Context, dnsAddr string) (string, error)
}

func NewFileManager(opts Options) *FileManager {
	m := &FileManager{
		ACME:        opts.ACME,
		LocalCA:     opts.LocalCA,
		Logger:      opts.Logger,
		Storage:     opts.Storage,
		TokenSource: opts.TokenSource,
	}
	return m
}

// EnsureServerKeyAndCSR creates server.key (PKCS#8) and server.csr with SANs if missing.
// This is used by globule to keep legacy files on disk for other tools.
func (m *FileManager) EnsureServerKeyAndCSR(dir, commonName, country, state, city, org string, dns []string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	keyPath := filepath.Join(dir, "server.key")
	csrPath := filepath.Join(dir, "server.csr")

	// 1) server.key (PKCS#8) if missing
	if !exists(keyPath) {
		priv, pkcs8, err := genECDSAKeyPKCS8() // ECDSA P-256
		if err != nil {
			return err
		}
		_ = priv // signer not needed now
		if err := writePEM(keyPath, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400); err != nil {
			return err
		}
	}

	// 2) server.csr if missing
	if !exists(csrPath) {
		blk, _, err := readPEMBlock(keyPath)
		if err != nil {
			return err
		}
		signer, err := parseAnyPrivateKey(blk)
		if err != nil {
			return err
		}

		// Build CSR with SANs
		subj := pkix.Name{
			CommonName:   commonName,
			Country:      []string{country},
			Province:     []string{state},
			Locality:     []string{city},
			Organization: []string{org},
		}
		tpl := &x509.CertificateRequest{
			Subject:  subj,
			DNSNames: dns,
		}
		csrDER, err := x509.CreateCertificateRequest(rand.Reader, tpl, signer)
		if err != nil {
			return fmt.Errorf("create CSR: %w", err)
		}
		if err := writePEM(csrPath, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}, 0o444); err != nil {
			return err
		}
	}
	return nil
}

func writeConcat(dst string, sources []string, mode os.FileMode) error {
	var out []byte
	for _, s := range sources {
		b, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		out = append(out, b...)
	}
	return os.WriteFile(dst, out, mode)
}


// helper: writePEM is defined in storage.go; readPEMBlock/parseAnyPrivateKey are in leaf.go
// helper: exists is in storage.go
// helper: loadPair, notAfter, RotateIfExpiring are in validate*.go
// helper: ACME flow lives in acme_lego.go
// helper: Local CA issuance lives in ca.go / leaf_issue.go

// A tiny serial generator if you need it elsewhere
func serial() *big.Int { return big.NewInt(time.Now().UnixNano()) }

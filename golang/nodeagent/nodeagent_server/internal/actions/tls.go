package actions

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

type tlsEnsureAction struct{}
type tlsCertValidAction struct{}

func (tlsEnsureAction) Name() string { return "tls.ensure" }

func (tlsEnsureAction) Validate(args *structpb.Struct) error {
	return nil
}

func (tlsEnsureAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	paths := tlsPaths(args)
	if err := os.MkdirAll(filepath.Dir(paths.fullchain), 0o755); err != nil {
		return "", fmt.Errorf("create tls dir: %w", err)
	}
	if _, err := os.Stat(paths.fullchain); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("missing TLS material at %s; provide cert/key or enable ACME", paths.fullchain)
		}
		return "", fmt.Errorf("stat fullchain: %w", err)
	}
	if _, err := os.Stat(paths.privkey); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("missing TLS material at %s; provide cert/key or enable ACME", paths.privkey)
		}
		return "", fmt.Errorf("stat privkey: %w", err)
	}
	return "tls assets present", nil
}

func (tlsCertValidAction) Name() string { return "tls.cert_valid_for_domain" }

func (tlsCertValidAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	fields := args.GetFields()
	if strings.TrimSpace(fields["domain"].GetStringValue()) == "" {
		return errors.New("domain is required")
	}
	if strings.TrimSpace(fields["cert_path"].GetStringValue()) == "" {
		return errors.New("cert_path is required")
	}
	return nil
}

func (tlsCertValidAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	domain := strings.TrimSpace(fields["domain"].GetStringValue())
	path := strings.TrimSpace(fields["cert_path"].GetStringValue())
	if domain == "" || path == "" {
		return "", errors.New("domain and cert_path are required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read cert: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return "", errors.New("failed to decode PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse cert: %w", err)
	}
	if timeNow := cert.NotAfter; timeNow.IsZero() {
		// no-op
	}
	if cert.NotAfter.IsZero() || cert.NotAfter.Before(cert.NotBefore) {
		return "", errors.New("certificate validity window invalid")
	}
	now := nowFunc()
	if now.Before(cert.NotBefore) {
		return "", errors.New("certificate not yet valid")
	}
	if now.After(cert.NotAfter) {
		return "", errors.New("certificate expired")
	}
	if err := cert.VerifyHostname(domain); err != nil {
		return "", fmt.Errorf("certificate not valid for %s: %w", domain, err)
	}
	return "certificate valid for domain", nil
}

var nowFunc = func() time.Time { return time.Now() }

func init() {
	Register(tlsEnsureAction{})
	Register(tlsCertValidAction{})
}

type tlsPathsSet struct {
	fullchain string
	privkey   string
}

func tlsPaths(args *structpb.Struct) tlsPathsSet {
	const (
		defaultCert = "/var/lib/globular/tls/fullchain.pem"
		defaultKey  = "/var/lib/globular/tls/privkey.pem"
	)
	if args == nil {
		return tlsPathsSet{fullchain: defaultCert, privkey: defaultKey}
	}
	fields := args.GetFields()
	cert := strings.TrimSpace(fields["fullchain_path"].GetStringValue())
	key := strings.TrimSpace(fields["privkey_path"].GetStringValue())
	if cert == "" {
		cert = defaultCert
	}
	if key == "" {
		key = defaultKey
	}
	return tlsPathsSet{fullchain: cert, privkey: key}
}

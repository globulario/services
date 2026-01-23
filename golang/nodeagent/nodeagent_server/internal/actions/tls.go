package actions

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

type tlsCertValidAction struct{}

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
	Register(tlsCertValidAction{})
}

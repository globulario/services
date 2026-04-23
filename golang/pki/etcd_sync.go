package pki

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
)

const (
	etcdCAKey     = "/globular/pki/ca.crt"
	etcdCAMetaKey = "/globular/pki/ca.meta.json"
)

type caMetadata struct {
	SHA256      string `json:"sha256"`
	UpdatedUnix int64  `json:"updated_unix"`
	Source      string `json:"source,omitempty"`
}

func caSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func parseCAMetadata(raw []byte) (*caMetadata, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var meta caMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	meta.SHA256 = strings.ToLower(strings.TrimSpace(meta.SHA256))
	return &meta, nil
}

func putCAMetadata(ctx context.Context, sha string, source string) error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return err
	}
	meta := caMetadata{
		SHA256:      strings.ToLower(strings.TrimSpace(sha)),
		UpdatedUnix: time.Now().Unix(),
		Source:      strings.TrimSpace(source),
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = cli.Put(ctx, etcdCAMetaKey, string(payload))
	return err
}

// syncCAFromEtcd pulls CA from etcd (if present) into the local PKI dir.
// If etcd is unreachable, it returns nil (soft failure) to allow existing local
// files to be used. It never overwrites etcd.
func (m *FileManager) syncCAFromEtcd(dir string) error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, etcdCAKey)
	if err != nil {
		return err
	}
	if resp.Count == 0 {
		return nil // nothing stored yet
	}

	caData := resp.Kvs[0].Value
	caSHA := caSHA256Hex(caData)

	// Validate metadata hash when present (defense against stale/tampered value).
	metaResp, err := cli.Get(ctx, etcdCAMetaKey)
	if err == nil && metaResp.Count > 0 {
		meta, parseErr := parseCAMetadata(metaResp.Kvs[0].Value)
		if parseErr != nil {
			if m != nil && m.Logger != nil {
				m.Logger.Warn("pki: invalid CA metadata in etcd; continuing with cert payload", "err", parseErr)
			}
		} else if meta != nil && meta.SHA256 != "" && meta.SHA256 != caSHA {
			return fmt.Errorf("etcd CA metadata hash mismatch: meta=%s payload=%s", meta.SHA256, caSHA)
		}
	}

	localCrt := caCrtPath(dir)
	localPem := caPemPath(dir)
	// If local exists and hashes match, nothing to do.
	if b, err := os.ReadFile(localCrt); err == nil {
		if sha256.Sum256(b) == sha256.Sum256(caData) {
			// Backfill metadata if missing.
			if err := putCAMetadata(ctx, caSHA, "sync-local-match"); err != nil && m != nil && m.Logger != nil {
				m.Logger.Warn("pki: failed to backfill CA metadata in etcd", "err", err)
			}
			return nil
		}
	}
	if err := os.WriteFile(localCrt, caData, 0o444); err != nil {
		return fmt.Errorf("write local ca.crt: %w", err)
	}
	// Keep ca.pem in sync for legacy consumers.
	if err := os.WriteFile(localPem, caData, 0o444); err != nil {
		return fmt.Errorf("write local ca.pem: %w", err)
	}
	if err := putCAMetadata(ctx, caSHA, "sync-write-local"); err != nil {
		if m != nil && m.Logger != nil {
			m.Logger.Warn("pki: failed to write CA metadata to etcd", "err", err)
		} else {
			slog.Warn("pki: failed to write CA metadata to etcd", "err", err)
		}
	}
	return nil
}

// publishCALocalIfEtcdEmpty publishes the local CA certificate to etcd only if
// etcd does not already contain a value. It never overwrites an existing etcd CA.
func (m *FileManager) publishCALocalIfEtcdEmpty(localCrt string) error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, etcdCAKey)
	if err != nil {
		return err
	}
	if resp.Count > 0 {
		return nil // already populated
	}

	data, err := os.ReadFile(localCrt)
	if err != nil {
		return fmt.Errorf("read local ca: %w", err)
	}
	_, err = cli.Put(ctx, etcdCAKey, string(data))
	if err != nil {
		return err
	}
	if err := putCAMetadata(ctx, caSHA256Hex(data), "publish-local-if-empty"); err != nil && m != nil && m.Logger != nil {
		m.Logger.Warn("pki: failed to write CA metadata after publish", "err", err)
	}
	return nil
}

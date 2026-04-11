package pki

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"github.com/globulario/services/golang/config"
)

const (
	etcdCAKey = "/globular/pki/ca.crt"
)

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
	localCrt := caCrtPath(dir)
	localPem := caPemPath(dir)
	// If local exists and hashes match, nothing to do.
	if b, err := os.ReadFile(localCrt); err == nil {
		if sha256.Sum256(b) == sha256.Sum256(caData) {
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
	return err
}

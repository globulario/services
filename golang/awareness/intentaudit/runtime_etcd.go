package intentaudit

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdProvider implements RuntimeEvidenceProvider using a live etcd connection.
type EtcdProvider struct {
	client  *clientv3.Client
	timeout time.Duration
}

// NewEtcdProvider creates a provider from the cluster's etcd client.
func NewEtcdProvider() (*EtcdProvider, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	return &EtcdProvider{client: cli, timeout: 5 * time.Second}, nil
}

// NewEtcdProviderFromClient creates a provider from an existing client.
func NewEtcdProviderFromClient(cli *clientv3.Client) *EtcdProvider {
	return &EtcdProvider{client: cli, timeout: 5 * time.Second}
}

func (p *EtcdProvider) GetJSON(ctx context.Context, key string) ([]byte, error) {
	ctx2, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	resp, err := p.client.Get(ctx2, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return resp.Kvs[0].Value, nil
}

func (p *EtcdProvider) ListKeys(ctx context.Context, prefix string) ([]string, error) {
	ctx2, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	resp, err := p.client.Get(ctx2, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		keys[i] = string(kv.Key)
	}
	return keys, nil
}

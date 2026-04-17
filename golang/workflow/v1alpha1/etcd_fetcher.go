package v1alpha1

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
)

const etcdWorkflowPrefix = "/globular/workflows/"

// EnableEtcdFetcher configures the package-level EtcdFetcher to read workflow
// definitions from etcd. Core workflows live in etcd so they're available even
// when MinIO is down — etcd is on every node and is always the first thing up.
func EnableEtcdFetcher() {
	EtcdFetcher = func(name string) ([]byte, error) {
		if name == "" {
			return nil, fmt.Errorf("workflow name is empty")
		}
		cli, err := config.GetEtcdClient()
		if err != nil {
			return nil, fmt.Errorf("etcd client: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		key := etcdWorkflowPrefix + name
		resp, err := cli.Get(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("etcd get %s: %w", key, err)
		}
		if len(resp.Kvs) == 0 {
			return nil, fmt.Errorf("workflow %q not found in etcd", name)
		}
		data := resp.Kvs[0].Value
		log.Printf("workflow: loaded %q from etcd (%d bytes)", name, len(data))
		return data, nil
	}
}

// SeedCoreWorkflows writes the core workflow definitions to etcd if they're
// missing. Called by the controller at startup. These are the workflows the
// cluster needs to function — reconcile, release, join, bootstrap.
// Service-owned workflows (compute, doctor) stay in MinIO.
//
// Idempotent: only writes if the key doesn't exist or the content changed.
func SeedCoreWorkflows(definitions map[string][]byte) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	seeded := 0
	for name, data := range definitions {
		key := etcdWorkflowPrefix + name
		resp, err := cli.Get(ctx, key)
		if err != nil {
			log.Printf("workflow-seed: etcd get %s failed: %v", key, err)
			continue
		}

		// Skip if already present and identical.
		if len(resp.Kvs) > 0 && string(resp.Kvs[0].Value) == string(data) {
			continue
		}

		if _, err := cli.Put(ctx, key, string(data)); err != nil {
			log.Printf("workflow-seed: etcd put %s failed: %v", key, err)
			continue
		}
		if len(resp.Kvs) == 0 {
			log.Printf("workflow-seed: created %s in etcd (%d bytes)", name, len(data))
		} else {
			log.Printf("workflow-seed: updated %s in etcd (%d bytes)", name, len(data))
		}
		seeded++
	}

	if seeded > 0 {
		log.Printf("workflow-seed: seeded %d core workflow definitions to etcd", seeded)
	}
	return nil
}

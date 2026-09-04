package v1alpha1

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdWorkflowPrefix is the etcd key prefix under which all workflow YAML
// definitions are stored. Exported so the workflow server can list them.
const EtcdWorkflowPrefix = "/globular/workflows/"

// keep unexported alias for internal use within this file
const etcdWorkflowPrefix = EtcdWorkflowPrefix

// EtcdWorkflowLister is an optional callback that returns every workflow
// definition currently in etcd, keyed by the SAME bare workflow name
// EtcdFetcher takes and SeedCoreWorkflows writes — never a filename.
//
// It exists so a caller that wants "all core workflows" asks etcd instead of
// carrying its own list of which workflows are core. A second list is a second
// writer of that fact, and the two drift.
var EtcdWorkflowLister func() (map[string][]byte, error)

// EnableEtcdFetcher configures the package-level EtcdFetcher and
// EtcdWorkflowLister to read workflow definitions from etcd. Core workflows
// live in etcd so they're available even when MinIO is down — etcd is on every
// node and is always the first thing up.
func EnableEtcdFetcher() {
	EtcdWorkflowLister = func() (map[string][]byte, error) {
		cli, err := config.GetEtcdClient()
		if err != nil {
			return nil, fmt.Errorf("etcd client: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := cli.Get(ctx, etcdWorkflowPrefix, clientv3.WithPrefix())
		if err != nil {
			return nil, fmt.Errorf("etcd list %s: %w", etcdWorkflowPrefix, err)
		}
		definitions := make(map[string][]byte, len(resp.Kvs))
		for _, kv := range resp.Kvs {
			name := strings.TrimPrefix(string(kv.Key), etcdWorkflowPrefix)
			if name == "" {
				continue
			}
			definitions[name] = kv.Value
		}
		return definitions, nil
	}

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

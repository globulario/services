// Package installed_state provides read/write/query operations for the
// canonical installed-package registry stored in etcd.
//
// etcd key schema:
//
//	/globular/nodes/{node_id}/packages/{kind}/{name}
//
// Values are protojson-encoded node_agent.InstalledPackage records.
//
// This package is used by:
//   - Node Agent: writes records after successful lifecycle execution
//   - Cluster Controller: reads records for drift detection
//   - Gateway: reads records for admin UI queries
package installed_state

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	// keyPrefix is the etcd key prefix for installed packages.
	keyPrefix = "/globular/nodes/"

	// defaultTimeout for etcd operations.
	defaultTimeout = 5 * time.Second
)

// packageKey returns the etcd key for an installed package.
// Format: /globular/nodes/{node_id}/packages/{kind}/{name}
func packageKey(nodeID, kind, name string) string {
	return keyPrefix + nodeID + "/packages/" + strings.ToUpper(kind) + "/" + name
}

// nodePackagesPrefix returns the etcd prefix for all packages on a node.
func nodePackagesPrefix(nodeID string) string {
	return keyPrefix + nodeID + "/packages/"
}

// nodeKindPrefix returns the etcd prefix for packages of a given kind on a node.
func nodeKindPrefix(nodeID, kind string) string {
	return keyPrefix + nodeID + "/packages/" + strings.ToUpper(kind) + "/"
}

// WriteInstalledPackage writes (or overwrites) an installed package record in etcd.
// The record's UpdatedUnix is set to now if zero.
func WriteInstalledPackage(ctx context.Context, pkg *node_agentpb.InstalledPackage) error {
	if pkg.GetNodeId() == "" {
		return fmt.Errorf("installed_state: node_id is required")
	}
	if pkg.GetName() == "" {
		return fmt.Errorf("installed_state: name is required")
	}
	if pkg.GetKind() == "" {
		return fmt.Errorf("installed_state: kind is required")
	}

	if pkg.UpdatedUnix == 0 {
		pkg.UpdatedUnix = time.Now().Unix()
	}
	if pkg.InstalledUnix == 0 {
		pkg.InstalledUnix = pkg.UpdatedUnix
	}
	if pkg.Status == "" {
		pkg.Status = "installed"
	}

	data, err := protojson.Marshal(pkg)
	if err != nil {
		return fmt.Errorf("installed_state: marshal: %w", err)
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("installed_state: etcd client: %w", err)
	}
	defer cli.Close()

	key := packageKey(pkg.GetNodeId(), pkg.GetKind(), pkg.GetName())
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	_, err = cli.Put(tctx, key, string(data))
	if err != nil {
		return fmt.Errorf("installed_state: put %q: %w", key, err)
	}
	return nil
}

// GetInstalledPackage reads a single installed package record from etcd.
func GetInstalledPackage(ctx context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	defer cli.Close()

	key := packageKey(nodeID, kind, name)
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := cli.Get(tctx, key)
	if err != nil {
		return nil, fmt.Errorf("installed_state: get %q: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return unmarshalPackage(resp.Kvs[0].Value)
}

// ListInstalledPackages returns all installed packages on a node, optionally filtered by kind.
func ListInstalledPackages(ctx context.Context, nodeID, kind string) ([]*node_agentpb.InstalledPackage, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	defer cli.Close()

	prefix := nodePackagesPrefix(nodeID)
	if kind != "" {
		prefix = nodeKindPrefix(nodeID, kind)
	}

	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := cli.Get(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("installed_state: list %q: %w", prefix, err)
	}

	pkgs := make([]*node_agentpb.InstalledPackage, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		pkg, err := unmarshalPackage(kv.Value)
		if err != nil {
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// DeleteInstalledPackage removes an installed package record from etcd.
func DeleteInstalledPackage(ctx context.Context, nodeID, kind, name string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("installed_state: etcd client: %w", err)
	}
	defer cli.Close()

	key := packageKey(nodeID, kind, name)
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	_, err = cli.Delete(tctx, key)
	if err != nil {
		return fmt.Errorf("installed_state: delete %q: %w", key, err)
	}
	return nil
}

// ListAllNodes returns installed packages across all nodes, optionally filtered by kind.
// Useful for gateway admin endpoints.
func ListAllNodes(ctx context.Context, kind string) ([]*node_agentpb.InstalledPackage, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("installed_state: etcd client: %w", err)
	}
	defer cli.Close()

	prefix := keyPrefix
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := cli.Get(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("installed_state: list all: %w", err)
	}

	kindUpper := strings.ToUpper(kind)
	pkgs := make([]*node_agentpb.InstalledPackage, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		pkg, err := unmarshalPackage(kv.Value)
		if err != nil {
			continue
		}
		if kind != "" && strings.ToUpper(pkg.GetKind()) != kindUpper {
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func unmarshalPackage(data []byte) (*node_agentpb.InstalledPackage, error) {
	pkg := &node_agentpb.InstalledPackage{}
	if err := protojson.Unmarshal(data, pkg); err != nil {
		return nil, fmt.Errorf("installed_state: unmarshal: %w", err)
	}
	return pkg, nil
}

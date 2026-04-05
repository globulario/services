package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

const (
	appliedHashPrefix     = "globular/cluster/v1/applied_hash"
	appliedSvcHashPrefix  = "globular/cluster/v1/applied_hash_services"
	observedSvcHashPrefix = "globular/cluster/v1/observed_hash_services"
	failCountPrefix       = "globular/cluster/v1/fail_count"
	failCountSvcPrefix    = "globular/cluster/v1/fail_count_services"
)

func hashDesiredNetwork(net *cluster_controllerpb.DesiredNetwork) (string, error) {
	if net == nil {
		return "", nil
	}
	data, err := json.Marshal(net)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (srv *server) getNodeAppliedHash(ctx context.Context, nodeID string) (string, error) {
	if srv.kv == nil {
		return "", fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", appliedHashPrefix, nodeID)
	resp, err := srv.kv.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return string(resp.Kvs[0].Value), nil
}

func (srv *server) putNodeAppliedHash(ctx context.Context, nodeID, hash string) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", appliedHashPrefix, nodeID)
	_, err := srv.kv.Put(ctx, key, hash)
	return err
}

func (srv *server) getNodeFailureCount(ctx context.Context, nodeID string) (int, error) {
	if srv.kv == nil {
		return 0, fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", failCountPrefix, nodeID)
	resp, err := srv.kv.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if len(resp.Kvs) == 0 {
		return 0, nil
	}
	var count int
	if err := json.Unmarshal(resp.Kvs[0].Value, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (srv *server) putNodeFailureCount(ctx context.Context, nodeID string, count int) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", failCountPrefix, nodeID)
	data, err := json.Marshal(count)
	if err != nil {
		return err
	}
	_, err = srv.kv.Put(ctx, key, string(data))
	return err
}

func (srv *server) getNodeAppliedServiceHash(ctx context.Context, nodeID string) (string, error) {
	if srv.kv == nil {
		return "", fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", appliedSvcHashPrefix, nodeID)
	resp, err := srv.kv.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	val := string(resp.Kvs[0].Value)
	if val != "" && !strings.HasPrefix(val, "services:") {
		return "services:" + val, nil
	}
	return val, nil
}

func (srv *server) putNodeAppliedServiceHash(ctx context.Context, nodeID, hash string) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", appliedSvcHashPrefix, nodeID)
	_, err := srv.kv.Put(ctx, key, hash)
	return err
}

func (srv *server) putNodeObservedServiceHash(ctx context.Context, nodeID, hash string) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", observedSvcHashPrefix, nodeID)
	_, err := srv.kv.Put(ctx, key, hash)
	return err
}

func (srv *server) getNodeFailureCountServices(ctx context.Context, nodeID string) (int, error) {
	if srv.kv == nil {
		return 0, fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", failCountSvcPrefix, nodeID)
	resp, err := srv.kv.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	if len(resp.Kvs) == 0 {
		return 0, nil
	}
	var count int
	if err := json.Unmarshal(resp.Kvs[0].Value, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (srv *server) putNodeFailureCountServices(ctx context.Context, nodeID string, count int) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", failCountSvcPrefix, nodeID)
	data, err := json.Marshal(count)
	if err != nil {
		return err
	}
	_, err = srv.kv.Put(ctx, key, string(data))
	return err
}

func desiredNetworkToSpec(net *cluster_controllerpb.DesiredNetwork) *cluster_controllerpb.ClusterNetworkSpec {
	if net == nil {
		return nil
	}
	return &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain:    net.GetDomain(),
		Protocol:         net.GetProtocol(),
		PortHttp:         net.GetPortHttp(),
		PortHttps:        net.GetPortHttps(),
		AlternateDomains: append([]string(nil), net.GetAlternateDomains()...),
		AcmeEnabled:      net.GetAcmeEnabled(),
		AdminEmail:       net.GetAdminEmail(),
	}
}

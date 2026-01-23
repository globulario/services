package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	desiredStateKey      = "globular/cluster/v1/desired"
	desiredHashPrefix    = "globular/cluster/v1/desired_hash"
	desiredSvcHashPrefix = "globular/cluster/v1/desired_hash_services"
	appliedHashPrefix    = "globular/cluster/v1/applied_hash"
	planMetaPrefix       = "globular/cluster/v1/plan_meta"
	failCountPrefix      = "globular/cluster/v1/fail_count"
)

type planMeta struct {
	PlanId      string `json:"plan_id"`
	Generation  uint64 `json:"generation"`
	DesiredHash string `json:"desired_hash"`
	LastEmit    int64  `json:"last_emit_unix"`
}

func (srv *server) loadDesiredState(ctx context.Context) (*clustercontrollerpb.DesiredState, error) {
	if srv.kv == nil {
		return nil, fmt.Errorf("etcd client unavailable")
	}
	resp, err := srv.kv.Get(ctx, desiredStateKey)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var desired clustercontrollerpb.DesiredState
	if err := proto.Unmarshal(resp.Kvs[0].Value, &desired); err != nil {
		return nil, err
	}
	return &desired, nil
}

func (srv *server) saveDesiredState(ctx context.Context, desired *clustercontrollerpb.DesiredState) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	if desired == nil {
		return fmt.Errorf("desired state is nil")
	}
	data, err := proto.Marshal(desired)
	if err != nil {
		return err
	}
	_, err = srv.kv.Put(ctx, desiredStateKey, string(data))
	return err
}

func hashDesiredNetwork(net *clustercontrollerpb.DesiredNetwork) (string, error) {
	if net == nil {
		return "", nil
	}
	data, err := protojson.Marshal(net)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (srv *server) getNodeDesiredHash(ctx context.Context, nodeID string) (string, error) {
	if srv.kv == nil {
		return "", fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", desiredHashPrefix, nodeID)
	resp, err := srv.kv.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return string(resp.Kvs[0].Value), nil
}

func (srv *server) putNodeDesiredHash(ctx context.Context, nodeID, hash string) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", desiredHashPrefix, nodeID)
	_, err := srv.kv.Put(ctx, key, hash)
	return err
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

func (srv *server) getNodePlanMeta(ctx context.Context, nodeID string) (*planMeta, error) {
	if srv.kv == nil {
		return nil, fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", planMetaPrefix, nodeID)
	resp, err := srv.kv.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var meta planMeta
	if err := json.Unmarshal(resp.Kvs[0].Value, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (srv *server) putNodePlanMeta(ctx context.Context, nodeID string, meta *planMeta) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	if meta == nil {
		return fmt.Errorf("plan meta is nil")
	}
	key := fmt.Sprintf("%s/%s", planMetaPrefix, nodeID)
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = srv.kv.Put(ctx, key, string(data))
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

func (srv *server) ensureDesiredState(ctx context.Context) (*clustercontrollerpb.DesiredState, error) {
	if desired, err := srv.loadDesiredState(ctx); err == nil && desired != nil {
		return desired, nil
	} else if err != nil {
		return nil, err
	}
	now := timestamppb.Now()
	desired := &clustercontrollerpb.DesiredState{
		Generation: 1,
		UpdatedAt:  now,
		Network: &clustercontrollerpb.DesiredNetwork{
			Domain:           srv.cfg.ClusterDomain,
			Protocol:         "http",
			PortHttp:         80,
			PortHttps:        443,
			AcmeEnabled:      false,
			AdminEmail:       "",
			AlternateDomains: nil,
		},
	}
	if err := srv.saveDesiredState(ctx, desired); err != nil {
		return nil, err
	}
	return desired, nil
}

func desiredNetworkToSpec(net *clustercontrollerpb.DesiredNetwork) *clustercontrollerpb.ClusterNetworkSpec {
	if net == nil {
		return nil
	}
	return &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain:    net.GetDomain(),
		Protocol:         net.GetProtocol(),
		PortHttp:         net.GetPortHttp(),
		PortHttps:        net.GetPortHttps(),
		AlternateDomains: append([]string(nil), net.GetAlternateDomains()...),
		AcmeEnabled:      net.GetAcmeEnabled(),
		AdminEmail:       net.GetAdminEmail(),
	}
}

func (srv *server) applyDesiredNetwork(ctx context.Context, net *clustercontrollerpb.DesiredNetwork) (*clustercontrollerpb.DesiredState, error) {
	desired, err := srv.loadDesiredState(ctx)
	if err != nil {
		return nil, err
	}
	if desired == nil {
		desired = &clustercontrollerpb.DesiredState{}
	}
	changed := !proto.Equal(desired.GetNetwork(), net)
	desired.Network = proto.Clone(net).(*clustercontrollerpb.DesiredNetwork)
	if changed {
		desired.Generation++
	}
	if desired.Generation == 0 {
		desired.Generation = 1
	}
	desired.UpdatedAt = timestamppb.Now()
	if err := srv.saveDesiredState(ctx, desired); err != nil {
		return nil, err
	}
	// Keep legacy in-memory snapshot in sync.
	if netSpec := desiredNetworkToSpec(net); netSpec != nil {
		gen := computeNetworkGeneration(netSpec)
		srv.lock("desiredNetwork:snapshot")
		srv.state.ClusterNetworkSpec = netSpec
		srv.state.NetworkingGeneration = gen
		srv.unlock()
	}
	return desired, nil
}

func (srv *server) getNodeDesiredServiceHash(ctx context.Context, nodeID string) (string, error) {
	if srv.kv == nil {
		return "", fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", desiredSvcHashPrefix, nodeID)
	resp, err := srv.kv.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return string(resp.Kvs[0].Value), nil
}

func (srv *server) putNodeDesiredServiceHash(ctx context.Context, nodeID, hash string) error {
	if srv.kv == nil {
		return fmt.Errorf("etcd client unavailable")
	}
	key := fmt.Sprintf("%s/%s", desiredSvcHashPrefix, nodeID)
	_, err := srv.kv.Put(ctx, key, hash)
	return err
}

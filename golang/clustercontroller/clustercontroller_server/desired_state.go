package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	desiredStateKey   = "globular/cluster/v1/desired"
	desiredHashPrefix = "globular/cluster/v1/desired_hash"
)

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

func (srv *server) ensureDesiredState(ctx context.Context) (*clustercontrollerpb.DesiredState, error) {
	if desired, err := srv.loadDesiredState(ctx); err == nil && desired != nil {
		return desired, nil
	} else if err != nil {
		return nil, err
	}
	now := timestamppb.Now()
	return &clustercontrollerpb.DesiredState{
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
	}, nil
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

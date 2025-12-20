package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	"github.com/globulario/services/golang/plan/planpb"
)

const (
	planBaseKey       = "globular/plans/v1/nodes"
	currentPlanSuffix = "current"
	statusSuffix      = "status"
	historySuffix     = "history"
	PlanLockBaseKey   = "globular/plans/v1/locks"
)

// PlanStore exposes the minimal persistence interface for NodePlan and NodePlanStatus.
type PlanStore interface {
	PutCurrentPlan(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
	GetCurrentPlan(ctx context.Context, nodeID string) (*planpb.NodePlan, error)
	PutStatus(ctx context.Context, nodeID string, status *planpb.NodePlanStatus) error
	GetStatus(ctx context.Context, nodeID string) (*planpb.NodePlanStatus, error)
	AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
}

// EtcdPlanStore is a PlanStore backed by etcd.
type EtcdPlanStore struct {
	client *clientv3.Client
}

// NewEtcdPlanStore builds a new EtcdPlanStore using the provided etcd client.
func NewEtcdPlanStore(client *clientv3.Client) *EtcdPlanStore {
	return &EtcdPlanStore{client: client}
}

func (s *EtcdPlanStore) PutCurrentPlan(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	return s.putProto(ctx, currentPlanKey(nodeID), plan)
}

func (s *EtcdPlanStore) GetCurrentPlan(ctx context.Context, nodeID string) (*planpb.NodePlan, error) {
	var plan planpb.NodePlan
	if err := s.getProto(ctx, currentPlanKey(nodeID), &plan); err != nil {
		return nil, err
	}
	if plan.NodeId == "" {
		return nil, nil
	}
	return &plan, nil
}

func (s *EtcdPlanStore) PutStatus(ctx context.Context, nodeID string, status *planpb.NodePlanStatus) error {
	return s.putProto(ctx, statusKey(nodeID), status)
}

func (s *EtcdPlanStore) GetStatus(ctx context.Context, nodeID string) (*planpb.NodePlanStatus, error) {
	var status planpb.NodePlanStatus
	if err := s.getProto(ctx, statusKey(nodeID), &status); err != nil {
		return nil, err
	}
	if status.NodeId == "" {
		return nil, nil
	}
	return &status, nil
}

func (s *EtcdPlanStore) AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	key := historyKey(nodeID, plan.Generation)
	return s.putProto(ctx, key, plan)
}

func (s *EtcdPlanStore) Client() *clientv3.Client {
	return s.client
}

func (s *EtcdPlanStore) putProto(ctx context.Context, key string, message proto.Message) error {
	if message == nil {
		return fmt.Errorf("nil message for key %s", key)
	}
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	_, err = s.client.Put(ctx, key, string(data))
	return err
}

func (s *EtcdPlanStore) getProto(ctx context.Context, key string, message proto.Message) error {
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return err
	}
	if len(resp.Kvs) == 0 {
		return nil
	}
	return proto.Unmarshal(resp.Kvs[0].Value, message)
}

func currentPlanKey(nodeID string) string {
	return fmt.Sprintf("%s/%s/%s", planBaseKey, nodeID, currentPlanSuffix)
}

func statusKey(nodeID string) string {
	return fmt.Sprintf("%s/%s/%s", planBaseKey, nodeID, statusSuffix)
}

func historyKey(nodeID string, generation uint64) string {
	return fmt.Sprintf("%s/%s/%s/%d", planBaseKey, nodeID, historySuffix, generation)
}

// ComputePlanHash returns a hash of the plan's UnitActions for quick comparisons.
func ComputePlanHash(plan *planpb.NodePlan) (string, error) {
	if plan == nil {
		return "", nil
	}
	data, err := proto.Marshal(plan)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

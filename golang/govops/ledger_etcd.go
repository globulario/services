package govops

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/config"
	pb "github.com/globulario/services/golang/govops/governed_operationpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

// LedgerPrefix is the etcd key prefix under which operation ledger entries live.
// It mirrors the audittrail desired-write convention (/globular/audit/...).
const LedgerPrefix = "/globular/ops/ledger/"

// EtcdLedgerStore is an etcd-backed LedgerStore. Entries are append-only, keyed by
// timestamp + operation id so a prefix range read returns the full ledger.
type EtcdLedgerStore struct{}

// NewEtcdLedgerStore returns an etcd-backed ledger store.
func NewEtcdLedgerStore() *EtcdLedgerStore { return &EtcdLedgerStore{} }

// Put writes one ledger entry as protojson under LedgerPrefix.
func (EtcdLedgerStore) Put(ctx context.Context, e *pb.OperationLedgerEntry) error {
	if e == nil {
		return fmt.Errorf("ledger: nil entry")
	}
	data, err := protojson.Marshal(e)
	if err != nil {
		return fmt.Errorf("ledger: marshal: %w", err)
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("ledger: etcd client: %w", err)
	}
	// Key sorts by write time, then operation id for determinism within a tick.
	ts := time.Now().UTC().Format("20060102T150405.000000000")
	key := fmt.Sprintf("%s%s_%s", LedgerPrefix, ts, e.GetOperationId())
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := cli.Put(wctx, key, string(data)); err != nil {
		return fmt.Errorf("ledger: put: %w", err)
	}
	return nil
}

// List range-reads every ledger entry under LedgerPrefix.
func (EtcdLedgerStore) List(ctx context.Context) ([]*pb.OperationLedgerEntry, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("ledger: etcd client: %w", err)
	}
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(rctx, LedgerPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("ledger: get: %w", err)
	}
	out := make([]*pb.OperationLedgerEntry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var e pb.OperationLedgerEntry
		if err := protojson.Unmarshal(kv.Value, &e); err != nil {
			// A single corrupt record must not blind the whole ledger query.
			continue
		}
		out = append(out, &e)
	}
	return out, nil
}

var _ LedgerStore = EtcdLedgerStore{}

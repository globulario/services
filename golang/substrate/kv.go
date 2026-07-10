package substrate

import (
	"context"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// KV is the narrow store surface the dump/restore logic needs. It exists so
// the survival logic is unit-testable without an etcd server and so the
// package never grows an accidental dependency on cluster infrastructure —
// this code must run when the cluster is at its most broken.
type KV interface {
	// Range returns keys in [start, end), ascending, at most limit, read at
	// revision rev (0 = current head). It returns the head revision the read
	// was served at and whether more keys remain past the returned page.
	Range(ctx context.Context, start, end string, rev, limit int64) (kvs []*mvccpb.KeyValue, headRev int64, more bool, err error)

	// Get returns the current value of a single key (nil when absent) and the
	// head revision of the read.
	Get(ctx context.Context, key string) (*mvccpb.KeyValue, int64, error)

	// Put writes a key. Restore only ever writes without leases — restored
	// state must not evaporate.
	Put(ctx context.Context, key string, val []byte) error
}

// EtcdKV adapts a clientv3 client to KV. Serializable selects local
// (non-linearizable) reads: mandatory for dumping a quorum-less member —
// that capability is what makes rung-2 recovery possible at all.
type EtcdKV struct {
	Client       *clientv3.Client
	Serializable bool
}

func (e *EtcdKV) readOpts(end string, rev, limit int64) []clientv3.OpOption {
	opts := []clientv3.OpOption{
		clientv3.WithRange(end),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
		clientv3.WithLimit(limit),
	}
	if rev > 0 {
		opts = append(opts, clientv3.WithRev(rev))
	}
	if e.Serializable {
		opts = append(opts, clientv3.WithSerializable())
	}
	return opts
}

func (e *EtcdKV) Range(ctx context.Context, start, end string, rev, limit int64) ([]*mvccpb.KeyValue, int64, bool, error) {
	resp, err := e.Client.Get(ctx, start, e.readOpts(end, rev, limit)...)
	if err != nil {
		return nil, 0, false, err
	}
	return resp.Kvs, resp.Header.Revision, resp.More, nil
}

func (e *EtcdKV) Get(ctx context.Context, key string) (*mvccpb.KeyValue, int64, error) {
	var opts []clientv3.OpOption
	if e.Serializable {
		opts = append(opts, clientv3.WithSerializable())
	}
	resp, err := e.Client.Get(ctx, key, opts...)
	if err != nil {
		return nil, 0, err
	}
	if len(resp.Kvs) == 0 {
		return nil, resp.Header.Revision, nil
	}
	return resp.Kvs[0], resp.Header.Revision, nil
}

func (e *EtcdKV) Put(ctx context.Context, key string, val []byte) error {
	_, err := e.Client.Put(ctx, key, string(val))
	return err
}

// rangeEnd computes the exclusive upper bound for a prefix scan (the same
// construction as clientv3.GetPrefixRangeEnd, kept local so the pure logic
// has no clientv3 dependency at call sites).
func rangeEnd(prefix string) string {
	b := []byte(prefix)
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < 0xff {
			b[i]++
			return string(b[:i+1])
		}
	}
	return "\x00" // whole keyspace
}

// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=etcd_query_helper_utilities
// @awareness risk=medium
package rules

import (
	"context"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
)

// PrefixScan returns all keys under prefix from etcd using a keys-only range
// scan. Returns an empty (non-nil) slice when the prefix exists but has no
// children. Errors are returned unwrapped so callers can use errors.Is and
// mapCheckErr.
func PrefixScan(ctx context.Context, prefix string) ([]*mvccpb.KeyValue, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, err
	}
	resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	if resp.Kvs == nil {
		return []*mvccpb.KeyValue{}, nil
	}
	return resp.Kvs, nil
}

// mapCheckErr maps a non-nil query error to InvariantStateCheckError.
// Returns an empty string when err is nil so callers can decide PASS or FAIL
// based on the data.
func mapCheckErr(err error) InvariantState {
	if err == nil {
		return ""
	}
	return InvariantStateCheckError
}

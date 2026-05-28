package main

import (
	"context"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/remediation"
)

// loadFailureRatePolicy reads the cluster-wide policy from etcd. On any
// error (etcd unreachable, missing key, parse failure) returns defaults so
// throttling is always defined — missing policy must never become
// "unlimited retries". See docs/intent/remediation.failure_rate_policy.yaml.
func loadFailureRatePolicy(ctx context.Context) *remediation.FailureRatePolicy {
	getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return remediation.LoadFromEtcd(getCtx, etcdPolicyGetter{})
}

// etcdPolicyGetter adapts the cluster etcd client to remediation.PolicyEtcdGetter.
type etcdPolicyGetter struct{}

func (etcdPolicyGetter) Get(ctx context.Context, key string) ([]byte, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, err
	}
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return resp.Kvs[0].Value, nil
}

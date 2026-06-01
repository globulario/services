package security

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	peerPublicKeyPrefix   = "/globular/security/public_keys"
	peerPublicKeyCurrent  = "current"
	keyringFetchCooldown  = 10 * time.Second
	keyringRequestTimeout = 2 * time.Second
)

var (
	keyringFetchRetryAfter sync.Map // map[issuer/kid]time.Time
)

var (
	fetchPeerPublicKeyFromCluster = fetchPeerPublicKeyFromClusterEtcd
	publishPeerPublicKeyToCluster = publishPeerPublicKeyToClusterEtcd
)

func peerPublicKeyIssuerPath(issuer string) string {
	return path.Join(peerPublicKeyPrefix, normID(issuer))
}

func peerPublicKeyEntryPath(issuer, kid string) string {
	entry := peerPublicKeyCurrent
	if strings.TrimSpace(kid) != "" {
		entry = kid
	}
	return path.Join(peerPublicKeyIssuerPath(issuer), entry)
}

func canAttemptKeyFetch(cacheKey string, now time.Time) bool {
	nextAny, ok := keyringFetchRetryAfter.Load(cacheKey)
	if !ok {
		return true
	}
	next, ok := nextAny.(time.Time)
	return !ok || now.After(next)
}

func noteKeyFetchResult(cacheKey string, err error, now time.Time) {
	if err == nil {
		keyringFetchRetryAfter.Delete(cacheKey)
		return
	}
	keyringFetchRetryAfter.Store(cacheKey, now.Add(keyringFetchCooldown))
}

func fetchPeerPublicKeyFromClusterEtcd(issuer, kid string) ([]byte, error) {
	cacheKey := issuer + "/" + kid
	now := time.Now()
	if !canAttemptKeyFetch(cacheKey, now) {
		return nil, fmt.Errorf("peer key fetch in cooldown for %s", cacheKey)
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		noteKeyFetchResult(cacheKey, err, now)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), keyringRequestTimeout)
	defer cancel()

	lookup := []string{peerPublicKeyEntryPath(issuer, kid)}
	if strings.TrimSpace(kid) != "" {
		lookup = append(lookup, peerPublicKeyEntryPath(issuer, ""))
	}

	for _, key := range lookup {
		res, getErr := cli.Get(ctx, key, clientv3.WithSerializable())
		if getErr != nil {
			noteKeyFetchResult(cacheKey, getErr, now)
			return nil, getErr
		}
		if len(res.Kvs) == 1 && len(res.Kvs[0].Value) > 0 {
			noteKeyFetchResult(cacheKey, nil, now)
			return append([]byte(nil), res.Kvs[0].Value...), nil
		}
	}

	missErr := fmt.Errorf("peer public key not found for issuer=%s kid=%s", issuer, kid)
	noteKeyFetchResult(cacheKey, missErr, now)
	return nil, missErr
}

func publishPeerPublicKeyToClusterEtcd(issuer, kid string, encPub []byte) error {
	if strings.TrimSpace(issuer) == "" || len(encPub) == 0 {
		return nil
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), keyringRequestTimeout)
	defer cancel()

	ops := []clientv3.Op{
		clientv3.OpPut(peerPublicKeyEntryPath(issuer, ""), string(encPub)),
	}
	if strings.TrimSpace(kid) != "" {
		ops = append(ops, clientv3.OpPut(peerPublicKeyEntryPath(issuer, kid), string(encPub)))
	}

	_, err = cli.Txn(ctx).Then(ops...).Commit()
	return err
}

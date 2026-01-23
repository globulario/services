package resourcestore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

type etcdStore struct {
	cli *clientv3.Client
}

const resourceKeyPrefix = "/globular/resources"

func NewEtcdStore(cli *clientv3.Client) Store {
	return &etcdStore{cli: cli}
}

func keyFor(typ, name string) string {
	return fmt.Sprintf("%s/%s/%s", resourceKeyPrefix, typ, name)
}

func (s *etcdStore) Get(ctx context.Context, typ, name string) (interface{}, string, error) {
	resp, err := s.cli.Get(ctx, keyFor(typ, name))
	if err != nil || len(resp.Kvs) == 0 {
		return nil, "", err
	}
	obj, err := decodeObject(typ, resp.Kvs[0].Value)
	if err != nil {
		return nil, "", err
	}
	return obj, fmt.Sprintf("%d", resp.Kvs[0].ModRevision), nil
}

func (s *etcdStore) List(ctx context.Context, typ, prefix string) ([]interface{}, string, error) {
	resp, err := s.cli.Get(ctx, fmt.Sprintf("%s/%s/", resourceKeyPrefix, typ), clientv3.WithPrefix())
	if err != nil {
		return nil, "", err
	}
	items := make([]interface{}, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		obj, err := decodeObject(typ, kv.Value)
		if err != nil {
			return nil, "", err
		}
		items = append(items, obj)
	}
	return items, fmt.Sprintf("%d", resp.Header.Revision), nil
}

func (s *etcdStore) Apply(ctx context.Context, typ string, obj interface{}) (interface{}, error) {
	meta := extractMeta(obj)
	if meta == nil || meta.Name == "" {
		return nil, fmt.Errorf("object meta.name required")
	}
	// Load existing to compute generation and resource_version.
	prevObj, prevRV, _ := s.Get(ctx, typ, meta.Name)
	prevHash := ""
	prevGen := int64(0)
	if prevObj != nil {
		prevHash, _ = hashSpec(prevObj)
		prevGen = extractMeta(prevObj).Generation
	}
	specHash, err := hashSpec(obj)
	if err != nil {
		return nil, err
	}
	meta.Generation = 1
	if prevGen > 0 {
		meta.Generation = prevGen
		if specHash != prevHash {
			meta.Generation++
		}
	}
	if status := extractStatus(obj); status == nil {
		setStatus(obj, &clustercontrollerpb.ObjectStatus{})
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	resp, err := s.cli.Put(ctx, keyFor(typ, meta.Name), string(b))
	if err != nil {
		return nil, err
	}
	meta.ResourceVersion = fmt.Sprintf("%d", resp.Header.Revision)
	// Re-encode with RV.
	b, _ = json.Marshal(obj)
	_, _ = s.cli.Put(ctx, keyFor(typ, meta.Name), string(b))
	_ = prevRV
	return decodeObject(typ, b)
}

func (s *etcdStore) Delete(ctx context.Context, typ, name string) error {
	_, err := s.cli.Delete(ctx, keyFor(typ, name))
	return err
}

func (s *etcdStore) Watch(ctx context.Context, typ, prefix, fromRV string) (<-chan Event, error) {
	out := make(chan Event, 16)
	rev := int64(0)
	if fromRV != "" {
		if v, err := strconv.ParseInt(fromRV, 10, 64); err == nil {
			rev = v + 1
		}
	}
	wch := s.cli.Watch(ctx, fmt.Sprintf("%s/%s/", resourceKeyPrefix, typ), clientv3.WithPrefix(), clientv3.WithRev(rev))
	go func() {
		defer close(out)
		currentRev := rev
		for {
			select {
			case <-ctx.Done():
				return
			case wr, ok := <-wch:
				if !ok {
					return
				}
				if wr.Err() == rpctypes.ErrCompacted {
					// Resync
					resp, err := s.cli.Get(ctx, fmt.Sprintf("%s/%s/", resourceKeyPrefix, typ), clientv3.WithPrefix())
					if err != nil {
						continue
					}
					currentRev = resp.Header.Revision + 1
					wch = s.cli.Watch(ctx, fmt.Sprintf("%s/%s/", resourceKeyPrefix, typ), clientv3.WithPrefix(), clientv3.WithRev(currentRev))
					continue
				}
				for _, ev := range wr.Events {
					obj, err := decodeObject(typ, ev.Kv.Value)
					if err != nil {
						continue
					}
					evtType := EventModified
					if ev.Type == clientv3.EventTypePut && ev.IsCreate() {
						evtType = EventAdded
					}
					if ev.Type == clientv3.EventTypeDelete {
						evtType = EventDeleted
					}
					out <- Event{
						Type:            evtType,
						ResourceVersion: fmt.Sprintf("%d", ev.Kv.ModRevision),
						Object:          obj,
					}
					currentRev = ev.Kv.ModRevision + 1
				}
			}
		}
	}()
	return out, nil
}

func (s *etcdStore) UpdateStatus(ctx context.Context, typ, name string, status *clustercontrollerpb.ObjectStatus) (interface{}, error) {
	obj, _, err := s.Get(ctx, typ, name)
	if err != nil || obj == nil {
		return nil, fmt.Errorf("not found")
	}
	setStatus(obj, status)
	return s.Apply(ctx, typ, obj)
}

func decodeObject(typ string, data []byte) (interface{}, error) {
	switch typ {
	case "ClusterNetwork":
		obj := &clustercontrollerpb.ClusterNetwork{}
		if err := json.Unmarshal(data, obj); err != nil {
			return nil, err
		}
		return obj, nil
	case "ServiceDesiredVersion":
		obj := &clustercontrollerpb.ServiceDesiredVersion{}
		if err := json.Unmarshal(data, obj); err != nil {
			return nil, err
		}
		return obj, nil
	case "Node":
		obj := &clustercontrollerpb.Node{}
		if err := json.Unmarshal(data, obj); err != nil {
			return nil, err
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unknown type %s", typ)
	}
}

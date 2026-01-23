package resourcestore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

const (
	EventAdded    = "ADDED"
	EventModified = "MODIFIED"
	EventDeleted  = "DELETED"
)

type Event struct {
	Type            string
	ResourceVersion string
	Object          interface{}
}

type Store interface {
	Get(ctx context.Context, typ, name string) (interface{}, string, error)
	List(ctx context.Context, typ, prefix string) ([]interface{}, string, error)
	Apply(ctx context.Context, typ string, obj interface{}) (interface{}, error)
	Delete(ctx context.Context, typ, name string) error
	Watch(ctx context.Context, typ, prefix, fromRV string) (<-chan Event, error)
	UpdateStatus(ctx context.Context, typ, name string, status *clustercontrollerpb.ObjectStatus) (interface{}, error)
}

type storedResource struct {
	obj        interface{}
	specHash   string
	generation int64
	rv         string
}

type memStore struct {
	mu       sync.RWMutex
	rv       int64
	objects  map[string]map[string]*storedResource
	watchers map[*watchRegistration]struct{}
	events   map[string][]Event
}

type watchRegistration struct {
	typ    string
	prefix string
	ch     chan Event
	ctx    context.Context
}

func NewMemStore() Store {
	return &memStore{
		objects:  make(map[string]map[string]*storedResource),
		watchers: make(map[*watchRegistration]struct{}),
		events:   make(map[string][]Event),
	}
}

func (s *memStore) nextRV() string {
	s.rv++
	return fmt.Sprintf("%d", s.rv)
}

func (s *memStore) Get(ctx context.Context, typ, name string) (interface{}, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if objs, ok := s.objects[typ]; ok {
		if res, ok := objs[name]; ok {
			return deepCopy(res.obj), res.rv, nil
		}
	}
	return nil, "", nil
}

func (s *memStore) List(ctx context.Context, typ, prefix string) ([]interface{}, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []interface{}
	for name, res := range s.objects[typ] {
		if prefix == "" || len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			out = append(out, deepCopy(res.obj))
		}
	}
	return out, fmt.Sprintf("%d", s.rv), nil
}

func (s *memStore) Apply(ctx context.Context, typ string, obj interface{}) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := extractMeta(obj)
	if meta == nil || meta.Name == "" {
		return nil, fmt.Errorf("object meta.name required")
	}
	specHash, err := hashSpec(obj)
	if err != nil {
		return nil, err
	}
	objs := s.objects[typ]
	if objs == nil {
		objs = make(map[string]*storedResource)
		s.objects[typ] = objs
	}
	prev, exists := objs[meta.Name]
	generation := int64(1)
	if exists {
		generation = prev.generation
		if prev.specHash != specHash {
			generation++
		}
	}
	meta.Generation = generation
	meta.ResourceVersion = s.nextRV()
	status := extractStatus(obj)
	if status == nil {
		setStatus(obj, &clustercontrollerpb.ObjectStatus{})
		status = extractStatus(obj)
	}
	res := &storedResource{
		obj:        deepCopy(obj),
		specHash:   specHash,
		generation: generation,
		rv:         meta.ResourceVersion,
	}
	objs[meta.Name] = res
	evt := Event{
		Type: func() string {
			if exists {
				return EventModified
			}
			return EventAdded
		}(),
		ResourceVersion: meta.ResourceVersion,
		Object:          deepCopy(obj),
	}
	s.recordEvent(evt, typ)
	s.broadcast(evt, typ, meta.Name)
	return deepCopy(obj), nil
}

func (s *memStore) Delete(ctx context.Context, typ, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	objs := s.objects[typ]
	if objs == nil {
		return nil
	}
	if res, ok := objs[name]; ok {
		delete(objs, name)
		rv := s.nextRV()
		evt := Event{
			Type:            EventDeleted,
			ResourceVersion: rv,
			Object:          deepCopy(res.obj),
		}
		s.recordEvent(evt, typ)
		s.broadcast(evt, typ, name)
	}
	return nil
}

func (s *memStore) Watch(ctx context.Context, typ, prefix, fromRV string) (<-chan Event, error) {
	ch := make(chan Event, 16)
	reg := &watchRegistration{typ: typ, prefix: prefix, ch: ch, ctx: ctx}
	s.mu.Lock()
	s.watchers[reg] = struct{}{}
	s.mu.Unlock()
	// Replay events newer than fromRV.
	go func() {
		if fromRV != "" {
			if rv, err := strconv.ParseInt(fromRV, 10, 64); err == nil {
				s.mu.RLock()
				history := append([]Event(nil), s.events[typ]...)
				s.mu.RUnlock()
				for _, evt := range history {
					if parsedRV(evt.ResourceVersion) > rv {
						select {
						case ch <- evt:
						default:
						}
					}
				}
			}
		}
	}()
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		delete(s.watchers, reg)
		close(ch)
		s.mu.Unlock()
	}()
	return ch, nil
}

func (s *memStore) UpdateStatus(ctx context.Context, typ, name string, status *clustercontrollerpb.ObjectStatus) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	objs := s.objects[typ]
	if objs == nil {
		return nil, fmt.Errorf("not found")
	}
	res, ok := objs[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	objCopy := deepCopy(res.obj)
	setStatus(objCopy, status)
	meta := extractMeta(objCopy)
	meta.ResourceVersion = s.nextRV()
	objs[name] = &storedResource{
		obj:        deepCopy(objCopy),
		specHash:   res.specHash,
		generation: res.generation,
		rv:         meta.ResourceVersion,
	}
	evt := Event{
		Type:            EventModified,
		ResourceVersion: meta.ResourceVersion,
		Object:          deepCopy(objCopy),
	}
	s.recordEvent(evt, typ)
	s.broadcast(evt, typ, name)
	return deepCopy(objCopy), nil
}

func (s *memStore) broadcast(evt Event, typ, name string) {
	for reg := range s.watchers {
		if reg.typ != "" && reg.typ != typ {
			continue
		}
		if reg.prefix != "" && (len(name) < len(reg.prefix) || name[:len(reg.prefix)] != reg.prefix) {
			continue
		}
		select {
		case reg.ch <- evt:
		default:
		}
	}
}

func (s *memStore) recordEvent(evt Event, typ string) {
	s.events[typ] = append(s.events[typ], evt)
}

func extractMeta(obj interface{}) *clustercontrollerpb.ObjectMeta {
	switch o := obj.(type) {
	case *clustercontrollerpb.ClusterNetwork:
		return o.Meta
	case *clustercontrollerpb.ServiceDesiredVersion:
		return o.Meta
	case *clustercontrollerpb.Node:
		return o.Meta
	default:
		return nil
	}
}

func extractStatus(obj interface{}) *clustercontrollerpb.ObjectStatus {
	switch o := obj.(type) {
	case *clustercontrollerpb.ClusterNetwork:
		return o.Status
	case *clustercontrollerpb.ServiceDesiredVersion:
		return o.Status
	case *clustercontrollerpb.Node:
		return o.Status
	default:
		return nil
	}
}

func setStatus(obj interface{}, st *clustercontrollerpb.ObjectStatus) {
	switch o := obj.(type) {
	case *clustercontrollerpb.ClusterNetwork:
		o.Status = st
	case *clustercontrollerpb.ServiceDesiredVersion:
		o.Status = st
	case *clustercontrollerpb.Node:
		o.Status = st
	}
}

func specForHash(obj interface{}) interface{} {
	switch o := obj.(type) {
	case *clustercontrollerpb.ClusterNetwork:
		return o.Spec
	case *clustercontrollerpb.ServiceDesiredVersion:
		return o.Spec
	case *clustercontrollerpb.Node:
		return o.Spec
	default:
		return nil
	}
}

func hashSpec(obj interface{}) (string, error) {
	spec := specForHash(obj)
	if spec == nil {
		return "", nil
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:]), nil
}

func deepCopy(obj interface{}) interface{} {
	if obj == nil {
		return nil
	}
	b, _ := json.Marshal(obj)
	var dst interface{}
	switch obj.(type) {
	case *clustercontrollerpb.ClusterNetwork:
		tmp := &clustercontrollerpb.ClusterNetwork{}
		_ = json.Unmarshal(b, tmp)
		dst = tmp
	case *clustercontrollerpb.ServiceDesiredVersion:
		tmp := &clustercontrollerpb.ServiceDesiredVersion{}
		_ = json.Unmarshal(b, tmp)
		dst = tmp
	case *clustercontrollerpb.Node:
		tmp := &clustercontrollerpb.Node{}
		_ = json.Unmarshal(b, tmp)
		dst = tmp
	default:
		dst = obj
	}
	return dst
}

func parsedRV(rv string) int64 {
	if rv == "" {
		return 0
	}
	v, err := strconv.ParseInt(rv, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

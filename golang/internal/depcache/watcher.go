package depcache

import (
	"context"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	watchErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "depcache",
		Name:      "watch_errors_total",
		Help:      "Total unexpected etcd watch channel closes per prefix.",
	}, []string{"prefix"})

	watchRestartsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "depcache",
		Name:      "watch_restarts_total",
		Help:      "Total watch goroutine restarts after failure per prefix.",
	}, []string{"prefix"})

	watchActiveGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "depcache",
		Name:      "watch_active",
		Help:      "1 when the watch goroutine is running for a prefix, 0 otherwise.",
	}, []string{"prefix"})
)

// WatchInvalidator maintains a long-lived etcd watch on a key prefix and
// calls invalidation callbacks when keys change.
//
// On watch channel close (unexpected): onFail is called (should call
// cache.InvalidateAll to prevent serving data from across the watch gap),
// then the watch restarts with exponential backoff.
//
// Metrics exposed per prefix:
//   depcache_watch_errors_total
//   depcache_watch_restarts_total
//   depcache_watch_active
type WatchInvalidator struct {
	client  *clientv3.Client
	prefix  string
	onEvent func(etcdKey string) // called per PUT/DELETE; may be nil
	onFail  func()               // called on watch failure; should call InvalidateAll

	// initialBackoff is the first retry delay. Defaults to 1s.
	// Override in tests for faster retries.
	initialBackoff time.Duration

	// watchFn replaces client.Watch in tests.
	watchFn func(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
}

// NewWatchInvalidator creates a WatchInvalidator.
//
// onEvent is called for every PUT or DELETE under prefix. It should map the
// etcd key path to a cache key and call cache.Invalidate(key). May be nil
// if only bulk invalidation via onFail is needed.
//
// onFail is called when the watch channel closes unexpectedly. It must call
// cache.InvalidateAll() to ensure no stale data is served across the gap.
func NewWatchInvalidator(
	client *clientv3.Client,
	prefix string,
	onEvent func(etcdKey string),
	onFail func(),
) *WatchInvalidator {
	return &WatchInvalidator{
		client:  client,
		prefix:  prefix,
		onEvent: onEvent,
		onFail:  onFail,
	}
}

// Start launches the watch goroutine. Returns immediately.
// The goroutine runs until ctx is cancelled.
func (w *WatchInvalidator) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *WatchInvalidator) loop(ctx context.Context) {
	backoff := w.initialBackoff
	if backoff == 0 {
		backoff = time.Second
	}
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			watchActiveGauge.WithLabelValues(w.prefix).Set(0)
			return
		}

		watchActiveGauge.WithLabelValues(w.prefix).Set(1)
		w.runWatch(ctx)
		watchActiveGauge.WithLabelValues(w.prefix).Set(0)

		// If context was cancelled, the watch ended normally — don't restart.
		if ctx.Err() != nil {
			return
		}

		// Watch ended unexpectedly: invalidate all cached data, then restart.
		watchErrorsTotal.WithLabelValues(w.prefix).Inc()
		log.Printf("depcache: watch on %q closed unexpectedly; invalidating cache, restarting in %s", w.prefix, backoff)
		w.onFail()

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}

		watchRestartsTotal.WithLabelValues(w.prefix).Inc()
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// runWatch runs a single watch session until the channel closes or ctx is cancelled.
func (w *WatchInvalidator) runWatch(ctx context.Context) {
	var watchCh clientv3.WatchChan
	if w.watchFn != nil {
		watchCh = w.watchFn(ctx, w.prefix, clientv3.WithPrefix())
	} else {
		watchCh = w.client.Watch(ctx, w.prefix, clientv3.WithPrefix())
	}

	for resp := range watchCh {
		if resp.Err() != nil {
			log.Printf("depcache: watch error on %q: %v", w.prefix, resp.Err())
			return
		}
		if w.onEvent == nil {
			continue
		}
		for _, ev := range resp.Events {
			w.onEvent(string(ev.Kv.Key))
		}
	}
}

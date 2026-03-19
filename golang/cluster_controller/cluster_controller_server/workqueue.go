package main

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string]int
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{attempts: make(map[string]int)}
}

func (rl *rateLimiter) backoff(key string) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[key]++
	attempt := rl.attempts[key]
	base := time.Second
	delay := base << (attempt - 1)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(time.Second))) // up to 1s jitter
	return delay + jitter
}

func (rl *rateLimiter) forget(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, key)
}

type workQueue struct {
	ch       chan string
	mu       sync.Mutex
	pending  map[string]struct{}
	inFlight map[string]struct{}
	dirty    map[string]struct{} // enqueued while in-flight — needs re-processing
	rl       *rateLimiter
}

func newWorkQueue(size int) *workQueue {
	return &workQueue{
		ch:       make(chan string, size),
		pending:  make(map[string]struct{}),
		inFlight: make(map[string]struct{}),
		dirty:    make(map[string]struct{}),
		rl:       newRateLimiter(),
	}
}

func (q *workQueue) Enqueue(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.pending[key]; ok {
		return
	}
	if _, ok := q.inFlight[key]; ok {
		// Mark dirty so Done() re-enqueues after the current processing finishes.
		// Without this, watch events that arrive while a key is being processed
		// (e.g. PENDING→RESOLVED transition) are silently lost.
		q.dirty[key] = struct{}{}
		return
	}
	q.pending[key] = struct{}{}
	select {
	case q.ch <- key:
	default:
		// best effort drop if queue full
		delete(q.pending, key)
	}
}

func (q *workQueue) EnqueueAfter(key string, d time.Duration) {
	time.AfterFunc(d, func() {
		q.Enqueue(key)
	})
}

func (q *workQueue) Get(ctx context.Context) (string, bool) {
	select {
	case <-ctx.Done():
		return "", false
	case key := <-q.ch:
		q.mu.Lock()
		delete(q.pending, key)
		q.inFlight[key] = struct{}{}
		q.mu.Unlock()
		return key, true
	}
}

func (q *workQueue) Done(key string, err error) {
	q.mu.Lock()
	delete(q.inFlight, key)
	wasDirty := false
	if _, ok := q.dirty[key]; ok {
		delete(q.dirty, key)
		wasDirty = true
	}
	q.mu.Unlock()

	if err == nil {
		q.rl.forget(key)
	} else {
		q.EnqueueAfter(key, q.rl.backoff(key))
		return
	}

	// If a new event arrived while we were processing this key,
	// re-enqueue immediately so the updated state is reconciled.
	if wasDirty {
		q.Enqueue(key)
	}
}

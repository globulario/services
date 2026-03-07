package main

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/event/eventpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultRingCapacity = 2000

type ringEntry struct {
	name string
	data []byte
	ts   time.Time
	seq  uint64
}

// eventRingBuffer is a bounded, mutex-guarded circular buffer of recent events.
type eventRingBuffer struct {
	mu      sync.Mutex
	buf     []ringEntry
	cap     int
	head    int // next write position
	count   int // entries currently stored (≤ cap)
	nextSeq atomic.Uint64
}

func newEventRingBuffer(capacity int) *eventRingBuffer {
	if capacity <= 0 {
		capacity = defaultRingCapacity
	}
	r := &eventRingBuffer{
		buf: make([]ringEntry, capacity),
		cap: capacity,
	}
	r.nextSeq.Store(1)
	return r
}

// append stores a new event and returns its sequence number.
func (r *eventRingBuffer) append(name string, data []byte) uint64 {
	seq := r.nextSeq.Add(1) - 1
	r.mu.Lock()
	r.buf[r.head] = ringEntry{
		name: name,
		data: append([]byte(nil), data...), // defensive copy
		ts:   time.Now(),
		seq:  seq,
	}
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
	r.mu.Unlock()
	return seq
}

// query returns events matching namePrefix with seq > afterSeq, up to limit.
func (r *eventRingBuffer) query(namePrefix string, afterSeq uint64, limit int) ([]*eventpb.PersistedEvent, uint64) {
	if limit <= 0 {
		limit = 100
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return nil, 0
	}

	// Walk the ring from oldest to newest.
	start := (r.head - r.count + r.cap) % r.cap
	var out []*eventpb.PersistedEvent
	var latestSeq uint64

	for i := 0; i < r.count; i++ {
		e := &r.buf[(start+i)%r.cap]
		if e.seq == 0 {
			continue
		}
		if e.seq > latestSeq {
			latestSeq = e.seq
		}
		if e.seq <= afterSeq {
			continue
		}
		if namePrefix != "" && !strings.HasPrefix(e.name, namePrefix) {
			continue
		}
		out = append(out, &eventpb.PersistedEvent{
			Name:     e.name,
			Data:     e.data,
			Ts:       timestamppb.New(e.ts),
			Sequence: e.seq,
		})
		if len(out) >= limit {
			break
		}
	}

	return out, latestSeq
}

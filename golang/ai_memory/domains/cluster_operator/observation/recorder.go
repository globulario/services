package observation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	behavioralpb "github.com/globulario/services/golang/ai_memory/behavioral_memorypb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recorder is the durable delivery path for governed observation bundles.
//
// # WHY THIS EXISTS
//
// RecordBundle (client.go) dials behavioral-memory once per bundle and closes
// the connection on return. That is a workable bridge but a poor permanent
// learning path: one connection per finding, no bounded queue, no retry, no
// backpressure accounting, and errors visible only to whoever reads the return
// value. A doctor sweep over a large cluster turns into a burst of concurrent
// dials, and a behavioral-memory blip silently loses the observations.
//
// The Recorder keeps one lazily established connection, a bounded queue, and a
// small worker pool, so the caller's hot path (GetClusterReport) never waits on
// behavioral persistence and never fans out unbounded goroutines.
//
// DESIGN CONSTRAINTS
//
//   - Enqueue never blocks. A full queue drops and counts; it must not stall
//     cluster diagnosis. Learning is subordinate to reporting.
//   - A failure to persist must be VISIBLE (counters, LastError) and never
//     silently discarded — degraded learning is a reportable state, not an
//     invisible one.
//   - The connection is invalidated and redialled after a transport failure so
//     a restarted behavioral-memory does not wedge delivery permanently.
type Recorder struct {
	opts RecorderOptions

	queue chan Bundle

	connMu sync.Mutex
	cc     *grpc.ClientConn

	// counters — atomic so Stats() never contends with the workers
	enqueued  atomic.Uint64
	persisted atomic.Uint64
	retried   atomic.Uint64
	dropped   atomic.Uint64
	failed    atomic.Uint64

	lastMu      sync.Mutex
	lastSuccess time.Time
	lastFailure time.Time
	lastError   string

	// writeBundle is the delivery seam. Production leaves it nil and goes through
	// writeOnce, which resolves the endpoint the one sanctioned way. Tests set it
	// to drive deterministic success and failure without depending on whether the
	// host happens to be running an AI-memory service.
	//
	// It is unexported and settable only from inside this package, so it adds no
	// configuration surface and cannot become an alternative production route.
	writeBundle func(Bundle) error

	stopOnce sync.Once
	stopped  chan struct{}
	wg       sync.WaitGroup
}

type RecorderOptions struct {
	// QueueSize bounds outstanding bundles. Beyond it, Enqueue drops.
	QueueSize int
	// Workers is the number of concurrent delivery goroutines.
	Workers int
	// MaxAttempts bounds retries per bundle (1 = no retry).
	MaxAttempts int
	// BaseBackoff is the first retry delay; it doubles per attempt.
	BaseBackoff time.Duration
	// CallTimeout bounds a single RecordSignal/RecordEvidence round trip.
	CallTimeout time.Duration
	// Addr overrides service resolution. Empty resolves from config.
	Addr string
}

func (o *RecorderOptions) applyDefaults() {
	if o.QueueSize <= 0 {
		o.QueueSize = 256
	}
	if o.Workers <= 0 {
		o.Workers = 2
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 3
	}
	if o.BaseBackoff <= 0 {
		o.BaseBackoff = 250 * time.Millisecond
	}
	if o.CallTimeout <= 0 {
		o.CallTimeout = 3 * time.Second
	}
}

// Stats is a point-in-time view of delivery health. Consumed by the
// learning-pipeline metrics and by the doctor rule that reports persistent
// degradation.
type Stats struct {
	Enqueued      uint64
	Persisted     uint64
	Retried       uint64
	Dropped       uint64
	Failed        uint64
	QueueDepth    int
	LastSuccessAt time.Time
	LastFailureAt time.Time
	LastError     string
}

// NewRecorder starts the worker pool. Call Close to drain and release.
func NewRecorder(opts RecorderOptions) *Recorder {
	opts.applyDefaults()
	r := &Recorder{
		opts:    opts,
		queue:   make(chan Bundle, opts.QueueSize),
		stopped: make(chan struct{}),
	}
	for i := 0; i < opts.Workers; i++ {
		r.wg.Add(1)
		go r.worker()
	}
	return r
}

// Enqueue offers a bundle for delivery. It NEVER blocks.
//
// Returns false when the bundle was rejected (invalid, queue full, or the
// recorder is closing). A false return is a counted, reportable event — the
// caller should continue its own work regardless.
func (r *Recorder) Enqueue(b Bundle) bool {
	if b.Signal.Project == "" || b.Signal.Domain == "" {
		r.dropped.Add(1)
		r.noteFailure(errors.New("observation bundle requires project and domain"))
		return false
	}
	select {
	case <-r.stopped:
		r.dropped.Add(1)
		return false
	default:
	}
	select {
	case r.queue <- b:
		r.enqueued.Add(1)
		return true
	default:
		// Queue full: drop rather than block the caller's report path.
		r.dropped.Add(1)
		r.noteFailure(fmt.Errorf("observation queue full (size %d) — dropping signal %s",
			r.opts.QueueSize, b.Signal.ID))
		return false
	}
}

func (r *Recorder) worker() {
	defer r.wg.Done()
	for {
		select {
		case <-r.stopped:
			// Drain what is already queued so a graceful Close does not lose
			// observations that were accepted.
			for {
				select {
				case b := <-r.queue:
					r.deliver(b)
				default:
					return
				}
			}
		case b := <-r.queue:
			r.deliver(b)
		}
	}
}

func (r *Recorder) deliver(b Bundle) {
	backoff := r.opts.BaseBackoff
	for attempt := 1; attempt <= r.opts.MaxAttempts; attempt++ {
		err := r.writeOnce(b)
		if err == nil {
			r.persisted.Add(1)
			r.noteSuccess()
			return
		}
		// A transport-level failure means the cached connection may be dead;
		// drop it so the next attempt redials rather than reusing a corpse.
		if isTransportError(err) {
			r.invalidateConn()
		}
		if attempt == r.opts.MaxAttempts {
			r.failed.Add(1)
			r.noteFailure(fmt.Errorf("signal %s after %d attempts: %w", b.Signal.ID, attempt, err))
			return
		}
		r.retried.Add(1)
		select {
		case <-time.After(backoff):
		case <-r.stopped:
			r.failed.Add(1)
			r.noteFailure(fmt.Errorf("signal %s abandoned on shutdown: %w", b.Signal.ID, err))
			return
		}
		backoff *= 2
	}
}

// writeOnce performs one full bundle write against a (possibly cached) conn.
func (r *Recorder) writeOnce(b Bundle) error {
	if r.writeBundle != nil {
		return r.writeBundle(b)
	}
	cc, err := r.conn()
	if err != nil {
		return err
	}
	client := behavioralpb.NewBehavioralMemoryServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), r.opts.CallTimeout)
	defer cancel()

	rsp, err := client.RecordSignal(ctx, &behavioralpb.RecordSignalRequest{Signal: signalToPB(b.Signal)})
	if err != nil {
		return fmt.Errorf("record signal: %w", err)
	}
	signalID := rsp.GetSignalId()
	for _, e := range b.Evidence {
		ev := e
		if ev.TargetKind == "" {
			ev.TargetKind = "signal"
		}
		if ev.TargetID == "" {
			ev.TargetID = signalID
		}
		if ev.ObservedFrom == "" {
			ev.ObservedFrom = signalID
		}
		if _, err := client.RecordEvidence(ctx, &behavioralpb.RecordEvidenceRequest{Evidence: evidenceToPB(ev)}); err != nil {
			return fmt.Errorf("record evidence %s: %w", ev.ID, err)
		}
	}
	return nil
}

// conn returns the cached connection, dialling lazily on first use.
func (r *Recorder) conn() (*grpc.ClientConn, error) {
	r.connMu.Lock()
	defer r.connMu.Unlock()
	if r.cc != nil {
		return r.cc, nil
	}
	addr := r.opts.Addr
	if addr == "" {
		addr = config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	}
	if addr == "" {
		return nil, errors.New("behavioral-memory endpoint not resolvable")
	}
	opts, err := globular.InternalDialOptions()
	if err != nil {
		return nil, fmt.Errorf("behavioral-memory dial options: %w", err)
	}
	// Lazy dial (no WithBlock), matching RecordBundle. Not migrated to
	// grpc.NewClient: it changes default target resolution (dns vs passthrough).
	cc, err := grpc.Dial(addr, opts...) //nolint:staticcheck // see note
	if err != nil {
		return nil, fmt.Errorf("behavioral-memory dial: %w", err)
	}
	r.cc = cc
	return cc, nil
}

func (r *Recorder) invalidateConn() {
	r.connMu.Lock()
	cc := r.cc
	r.cc = nil
	r.connMu.Unlock()
	if cc != nil {
		_ = cc.Close()
	}
}

func (r *Recorder) noteSuccess() {
	r.lastMu.Lock()
	r.lastSuccess = time.Now()
	r.lastMu.Unlock()
}

func (r *Recorder) noteFailure(err error) {
	r.lastMu.Lock()
	r.lastFailure = time.Now()
	if err != nil {
		r.lastError = err.Error()
	}
	r.lastMu.Unlock()
}

// Stats reports delivery health. Safe for concurrent use.
// RecorderHealth is the state an operator surface reports. It is derived here so
// every consumer reaches the same verdict from the same counters, rather than
// each re-deriving one and disagreeing.
type RecorderHealth string

const (
	// RecorderIdle means nothing has been attempted yet. Critically this is not
	// health: "no behavioral events occurred" must stay distinguishable from
	// "events were accepted and lost".
	RecorderIdle RecorderHealth = "no_delivery_attempted"
	// RecorderHealthy means the most recent outcome was a successful write.
	RecorderHealthy RecorderHealth = "healthy"
	// RecorderRecovered means delivery failed before but has since succeeded.
	RecorderRecovered RecorderHealth = "recovered"
	// RecorderQueuePressure means bundles were dropped before delivery.
	RecorderQueuePressure RecorderHealth = "queue_pressure"
	// RecorderFailing means bundles were accepted and then terminally lost.
	RecorderFailing RecorderHealth = "delivery_failing"
)

// Health classifies the recorder without requiring a new enqueue. A bundle that
// was accepted and later failed becomes visible on the next poll, which is the
// difference between a health surface and an error return.
func (s Stats) Health() RecorderHealth {
	switch {
	case s.Failed > 0 && !s.LastFailureAt.IsZero() && s.LastSuccessAt.After(s.LastFailureAt):
		return RecorderRecovered
	case s.Failed > 0:
		return RecorderFailing
	case s.Dropped > 0:
		return RecorderQueuePressure
	case s.Persisted > 0:
		return RecorderHealthy
	default:
		// No success, no failure, no drop — including the case where bundles are
		// enqueued but not yet delivered. Reporting that as healthy would be the
		// false green this issue exists to remove.
		return RecorderIdle
	}
}

func (r *Recorder) Stats() Stats {
	r.lastMu.Lock()
	ls, lf, le := r.lastSuccess, r.lastFailure, r.lastError
	r.lastMu.Unlock()
	return Stats{
		Enqueued:      r.enqueued.Load(),
		Persisted:     r.persisted.Load(),
		Retried:       r.retried.Load(),
		Dropped:       r.dropped.Load(),
		Failed:        r.failed.Load(),
		QueueDepth:    len(r.queue),
		LastSuccessAt: ls,
		LastFailureAt: lf,
		LastError:     le,
	}
}

// Close stops accepting work, drains what was already accepted, and releases
// the connection. Bounded by ctx.
func (r *Recorder) Close(ctx context.Context) error {
	r.stopOnce.Do(func() { close(r.stopped) })
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	var err error
	select {
	case <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}
	r.invalidateConn()
	return err
}

// isTransportError reports whether err suggests the connection itself is bad,
// as opposed to the server rejecting a well-delivered request. Only the former
// justifies discarding the cached connection.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled, codes.Unauthenticated:
		return true
	default:
		return false
	}
}

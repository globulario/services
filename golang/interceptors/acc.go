package interceptors

// Adaptive Concurrency Control (ACC) for the gRPC interceptor chain.
//
// Priority classification:
//
//	P0 — always admitted (gRPC health probes; never throttle)
//	P1-authz    — reserved pool (ValidateAction: cluster-wide auth multiplier)
//	P1-periodic — reserved pool (ReportNodeStatus, EmitWorkflowEvent: liveness heartbeats)
//	P1-control  — reserved pool (CompleteOperation, ExecuteWorkflow: workflow liveness)
//	P2          — adaptive AIMD window (everything else)
//
// Each P1 pool uses an atomicPool (atomic int64 inflight + capacity).
// Unlike the channel semaphore used in Phase 1, atomicPool supports live resize
// — the operator writes an ACCConfig to etcd and the change takes effect within
// seconds without a service restart.
//
// Warmup: P2 is fully open for the first max(5 min AND 200 samples, 30 min ceiling).
// Recalibration: every 5 min, baseline is adjusted using p25 RTT with four safety guards.
//
// # etcd config key
//
// /globular/system/acc/config — JSON-encoded ACCConfig.
// All fields are optional; omitted fields keep the running default.
// Changes are applied live by a background watcher goroutine.

import (
	"context"
	"encoding/json"
	"expvar"
	"log/slog"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/globulario/services/golang/config"
)

// ── ACC observability counters (expvar / /debug/vars) ─────────────────────────
var (
	accP2AdmittedVar = expvar.NewInt("acc.p2_admitted_total")
	accP2RejectedVar = expvar.NewInt("acc.p2_rejected_total")
	accP1AdmittedVar = expvar.NewInt("acc.p1_admitted_total")
	accP1RejectedVar = expvar.NewInt("acc.p1_rejected_total")
)

// ── Method classification maps ────────────────────────────────────────────────

// P0: always admitted — gRPC health probes must never be throttled.
var accP0Methods = map[string]bool{
	"/grpc.health.v1.Health/Check": true,
	"/grpc.health.v1.Health/Watch": true,
}

// P1-authz: ValidateAction is the cluster-wide auth multiplier.
// Full P0 exemption would allow unbounded load under cold-start storms.
// Bounded pool allows backpressure; callers queue to their deadline.
var accP1AuthzMethods = map[string]bool{
	"/rbac.RbacService/ValidateAction": true,
}

// P1-periodic: liveness heartbeats — loss blinds the controller or
// stalls workflow state machines.
var accP1PeriodicMethods = map[string]bool{
	"/cluster_controller.ClusterControllerService/ReportNodeStatus":  true,
	"/cluster_controller.ClusterControllerService/EmitWorkflowEvent": true,
}

// P1-control: workflow lifecycle completion — loss causes permanent hangs
// or convergence stalls.
var accP1ControlMethods = map[string]bool{
	"/cluster_controller.ClusterControllerService/CompleteOperation": true,
	"/workflow.WorkflowService/ExecuteWorkflow":                      true,
}

// ── Compile-time defaults ─────────────────────────────────────────────────────
// All are overridable at runtime via ACCConfig in etcd.

const (
	defaultP1AuthzSize    = int64(200)
	defaultP1PeriodicSize = int64(50)
	defaultP1ControlSize  = int64(10)

	defaultP2InitialWindow = int64(100)
	defaultP2MinWindow     = int64(10)
	defaultP2MaxWindow     = int64(2000)

	defaultAIMDIncrease         = int64(1)
	defaultAIMDIncreaseMult     = float64(1.5) // RTT < baseline × 1.5 → increase
	defaultAIMDDecreaseMult     = float64(2.0) // RTT > baseline × 2.0 → decrease
	defaultAIMDDecreaseRate     = float64(0.9) // multiplicative factor on decrease

	defaultRecalibIntervalSec = int64(300) // 5 min
	defaultRecalibAlpha       = float64(0.05)
	defaultRecalibMaxIncrease = float64(1.25) // sanity cap: reject if > 25% above baseline
	defaultRecalibLoadGate    = float64(0.60) // skip if inflight > 60% of window

	// Warmup
	warmupMinDuration = 5 * time.Minute
	warmupMinSamples  = int64(200)
	warmupMaxDuration = 30 * time.Minute

	// RTT ring
	rttRingCap = 512

	// etcd key
	accConfigEtcdKey = "/globular/system/acc/config"
)

// ── ACCConfig — operator-visible configuration schema ─────────────────────────

// ACCConfig holds all tunable ACC parameters. It is stored as JSON at
// accConfigEtcdKey and applied live by the config watcher. All fields are
// optional; zero values are ignored (existing defaults are kept).
type ACCConfig struct {
	// P1 pool sizes.
	P1AuthzSize    int64 `json:"p1_authz_size,omitempty"`
	P1PeriodicSize int64 `json:"p1_periodic_size,omitempty"`
	P1ControlSize  int64 `json:"p1_control_size,omitempty"`

	// P2 AIMD window bounds.
	P2MinWindow int64 `json:"p2_min_window,omitempty"`
	P2MaxWindow int64 `json:"p2_max_window,omitempty"`

	// AIMD thresholds (as multiples of baseline RTT).
	AIMDIncreaseThresholdMult float64 `json:"aimd_increase_threshold_mult,omitempty"`
	AIMDDecreaseThresholdMult float64 `json:"aimd_decrease_threshold_mult,omitempty"`
	AIMDDecreaseRate          float64 `json:"aimd_decrease_rate,omitempty"` // 0 < x < 1

	// Recalibration.
	RecalibIntervalSec int64   `json:"recalib_interval_sec,omitempty"`
	RecalibAlpha       float64 `json:"recalib_alpha,omitempty"`
	RecalibMaxIncrease float64 `json:"recalib_max_increase,omitempty"` // sanity cap multiplier
	RecalibLoadGate    float64 `json:"recalib_load_gate,omitempty"`    // 0..1
}

// ── atomicPool — resizable non-blocking admission pool ────────────────────────

// atomicPool is a concurrency-limited admission gate backed by two atomic
// counters.  Unlike a channel semaphore, capacity can be updated at any time
// with Store — the new limit takes effect on the very next tryAcquire call.
//
// Resize semantics: if capacity decreases and more slots are currently acquired
// than the new limit, existing holders finish normally (no eviction); the pool
// simply will not admit new requests until inflight drops below the new cap.
type atomicPool struct {
	inflight atomic.Int64
	capacity atomic.Int64
}

func newAtomicPool(cap int64) *atomicPool {
	p := &atomicPool{}
	p.capacity.Store(cap)
	return p
}

func (p *atomicPool) tryAcquire() bool {
	cap := p.capacity.Load()
	for {
		cur := p.inflight.Load()
		if cur >= cap {
			return false
		}
		if p.inflight.CompareAndSwap(cur, cur+1) {
			return true
		}
		// CAS lost to a concurrent acquire — retry immediately.
	}
}

func (p *atomicPool) release() {
	p.inflight.Add(-1)
}

// resize sets a new capacity. The change is visible to tryAcquire immediately.
func (p *atomicPool) resize(newCap int64) {
	if newCap > 0 {
		p.capacity.Store(newCap)
	}
}

func (p *atomicPool) cap() int64   { return p.capacity.Load() }
func (p *atomicPool) inUse() int64 { return p.inflight.Load() }

// ── RTT ring buffer ───────────────────────────────────────────────────────────

type rttRing struct {
	mu   sync.Mutex
	buf  []int64 // milliseconds
	cap  int
	head int
	size int
}

func newRTTRing() *rttRing {
	return &rttRing{buf: make([]int64, rttRingCap), cap: rttRingCap}
}

func (r *rttRing) push(ms int64) {
	r.mu.Lock()
	r.buf[r.head] = ms
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
	r.mu.Unlock()
}

func (r *rttRing) snapshot() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == 0 {
		return nil
	}
	out := make([]int64, r.size)
	copy(out, r.buf[:r.size])
	return out
}

func rttP25(samples []int64) int64 {
	sorted := make([]int64, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/4]
}

// ── float64 atomics ───────────────────────────────────────────────────────────
// Go's sync/atomic has no native float64; store as uint64 bit-pattern.

func atomicLoadFloat64(p *atomic.Uint64) float64 {
	return math.Float64frombits(p.Load())
}

func atomicStoreFloat64(p *atomic.Uint64, f float64) {
	p.Store(math.Float64bits(f))
}

// ── Adaptive controller ───────────────────────────────────────────────────────

type adaptiveController struct {
	// Warmup.
	startedAt   time.Time
	sampleCount atomic.Int64
	warmupDone  atomic.Bool

	// P2 AIMD.
	windowSize atomic.Int64 // current allowed concurrent P2
	inflight   atomic.Int64 // current P2 inflight
	minWindow  atomic.Int64
	maxWindow  atomic.Int64

	// AIMD tunable parameters (stored as atomic float64 bit-patterns).
	aIMDIncreaseMult atomic.Uint64 // RTT < baseline × mult → increase
	aIMDDecreaseMult atomic.Uint64 // RTT > baseline × mult → decrease
	aIMDDecreaseRate atomic.Uint64 // multiplicative factor (0 < x < 1)

	// RTT baseline (milliseconds).
	baselineRTT atomic.Int64
	ring        *rttRing

	// Recalibration.
	recalibMu          sync.Mutex
	recalibIntervalSec atomic.Int64
	recalibAlpha       atomic.Uint64
	recalibMaxIncrease atomic.Uint64
	recalibLoadGate    atomic.Uint64

	// P1 pools.
	p1Authz    *atomicPool
	p1Periodic *atomicPool
	p1Control  *atomicPool

	// Observability.
	p2Admitted atomic.Int64
	p2Rejected atomic.Int64
	p1Admitted atomic.Int64
	p1Rejected atomic.Int64
}

var (
	accInstance *adaptiveController
	accOnce     sync.Once
)

func newAdaptiveController() *adaptiveController {
	acc := &adaptiveController{
		startedAt:  time.Now(),
		ring:       newRTTRing(),
		p1Authz:    newAtomicPool(defaultP1AuthzSize),
		p1Periodic: newAtomicPool(defaultP1PeriodicSize),
		p1Control:  newAtomicPool(defaultP1ControlSize),
	}
	acc.windowSize.Store(defaultP2InitialWindow)
	acc.minWindow.Store(defaultP2MinWindow)
	acc.maxWindow.Store(defaultP2MaxWindow)
	atomicStoreFloat64(&acc.aIMDIncreaseMult, defaultAIMDIncreaseMult)
	atomicStoreFloat64(&acc.aIMDDecreaseMult, defaultAIMDDecreaseMult)
	atomicStoreFloat64(&acc.aIMDDecreaseRate, defaultAIMDDecreaseRate)
	acc.recalibIntervalSec.Store(defaultRecalibIntervalSec)
	atomicStoreFloat64(&acc.recalibAlpha, defaultRecalibAlpha)
	atomicStoreFloat64(&acc.recalibMaxIncrease, defaultRecalibMaxIncrease)
	atomicStoreFloat64(&acc.recalibLoadGate, defaultRecalibLoadGate)
	return acc
}

func getACC() *adaptiveController {
	accOnce.Do(func() {
		acc := newAdaptiveController()
		go acc.recalibLoop()
		go acc.runConfigWatcher()
		accInstance = acc
	})
	return accInstance
}

// ── Priority classification ───────────────────────────────────────────────────

type accPriority int

const (
	accPrioP0        accPriority = iota
	accPrioP1Authz
	accPrioP1Periodic
	accPrioP1Control
	accPrioP2
)

func accClassify(method string) accPriority {
	switch {
	case accP0Methods[method]:
		return accPrioP0
	case accP1AuthzMethods[method]:
		return accPrioP1Authz
	case accP1PeriodicMethods[method]:
		return accPrioP1Periodic
	case accP1ControlMethods[method]:
		return accPrioP1Control
	default:
		return accPrioP2
	}
}

// ── Warmup ────────────────────────────────────────────────────────────────────

func (acc *adaptiveController) isWarmedUp() bool {
	if acc.warmupDone.Load() {
		return true
	}
	elapsed := time.Since(acc.startedAt)
	if elapsed >= warmupMaxDuration {
		acc.exitWarmup()
		return true
	}
	if elapsed >= warmupMinDuration && acc.sampleCount.Load() >= warmupMinSamples {
		acc.exitWarmup()
		return true
	}
	return false
}

func (acc *adaptiveController) exitWarmup() {
	if !acc.warmupDone.CompareAndSwap(false, true) {
		return
	}
	samples := acc.ring.snapshot()
	baseline := int64(20)
	if len(samples) > 0 {
		if p := rttP25(samples); p > 0 {
			baseline = p
		}
	}
	acc.baselineRTT.Store(baseline)
	slog.Info("acc: warmup complete",
		"baseline_rtt_ms", baseline,
		"samples", len(samples),
		"elapsed", time.Since(acc.startedAt).Round(time.Second),
	)
}

// ── Admission ─────────────────────────────────────────────────────────────────

func accAdmit(ctx context.Context, method string) (release func(), err error) {
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}

	acc := getACC()

	switch accClassify(method) {
	case accPrioP0:
		return func() {}, nil

	case accPrioP1Authz:
		if acc.p1Authz.tryAcquire() {
			acc.p1Admitted.Add(1)
			accP1AdmittedVar.Add(1)
			return func() { acc.p1Authz.release() }, nil
		}
		acc.p1Rejected.Add(1)
		accP1RejectedVar.Add(1)
		slog.Warn("acc: P1-authz pool exhausted", "method", method,
			"capacity", acc.p1Authz.cap(), "inflight", acc.p1Authz.inUse())
		return nil, status.Errorf(codes.ResourceExhausted,
			"server overloaded: authorization pool exhausted")

	case accPrioP1Periodic:
		if acc.p1Periodic.tryAcquire() {
			acc.p1Admitted.Add(1)
			accP1AdmittedVar.Add(1)
			return func() { acc.p1Periodic.release() }, nil
		}
		acc.p1Rejected.Add(1)
		accP1RejectedVar.Add(1)
		slog.Warn("acc: P1-periodic pool exhausted", "method", method,
			"capacity", acc.p1Periodic.cap(), "inflight", acc.p1Periodic.inUse())
		return nil, status.Errorf(codes.ResourceExhausted,
			"server overloaded: periodic pool exhausted")

	case accPrioP1Control:
		if acc.p1Control.tryAcquire() {
			acc.p1Admitted.Add(1)
			accP1AdmittedVar.Add(1)
			return func() { acc.p1Control.release() }, nil
		}
		acc.p1Rejected.Add(1)
		accP1RejectedVar.Add(1)
		slog.Warn("acc: P1-control pool exhausted", "method", method,
			"capacity", acc.p1Control.cap(), "inflight", acc.p1Control.inUse())
		return nil, status.Errorf(codes.ResourceExhausted,
			"server overloaded: control pool exhausted")

	default:
		return acc.admitP2(method)
	}
}

func (acc *adaptiveController) admitP2(method string) (func(), error) {
	if !acc.isWarmedUp() {
		acc.inflight.Add(1)
		return func() { acc.inflight.Add(-1) }, nil
	}

	window := acc.windowSize.Load()
	if acc.inflight.Load() >= window {
		acc.p2Rejected.Add(1)
		accP2RejectedVar.Add(1)
		slog.Debug("acc: P2 rejected — window full",
			"method", method,
			"inflight", acc.inflight.Load(),
			"window", window,
		)
		return nil, status.Errorf(codes.ResourceExhausted,
			"server overloaded: too many concurrent requests")
	}

	acc.inflight.Add(1)
	acc.p2Admitted.Add(1)
	accP2AdmittedVar.Add(1)
	return func() { acc.inflight.Add(-1) }, nil
}

// ── RTT recording and AIMD ────────────────────────────────────────────────────

func accRecordRTT(method string, d time.Duration) {
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	acc := getACC()
	acc.ring.push(ms)
	acc.sampleCount.Add(1)

	if accClassify(method) != accPrioP2 || !acc.isWarmedUp() {
		return
	}

	baseline := acc.baselineRTT.Load()
	if baseline <= 0 {
		return
	}

	increaseMult := atomicLoadFloat64(&acc.aIMDIncreaseMult)
	decreaseMult := atomicLoadFloat64(&acc.aIMDDecreaseMult)
	decreaseRate := atomicLoadFloat64(&acc.aIMDDecreaseRate)
	minW := acc.minWindow.Load()
	maxW := acc.maxWindow.Load()
	current := acc.windowSize.Load()

	switch {
	case float64(ms) < float64(baseline)*increaseMult:
		if next := current + defaultAIMDIncrease; next <= maxW {
			acc.windowSize.CompareAndSwap(current, next)
		}
	case float64(ms) > float64(baseline)*decreaseMult:
		next := int64(float64(current) * decreaseRate)
		if next < minW {
			next = minW
		}
		if acc.windowSize.CompareAndSwap(current, next) {
			slog.Info("acc: AIMD decrease",
				"method", method,
				"rtt_ms", ms,
				"baseline_ms", baseline,
				"window", current,
				"new_window", next,
			)
		}
	}
}

// ── Recalibration ─────────────────────────────────────────────────────────────

func (acc *adaptiveController) recalibLoop() {
	// Read initial interval; rebuild ticker when config changes.
	interval := time.Duration(acc.recalibIntervalSec.Load()) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		acc.maybeRecalib()
		// Adjust ticker if interval changed.
		if newInterval := time.Duration(acc.recalibIntervalSec.Load()) * time.Second; newInterval != interval {
			interval = newInterval
			ticker.Reset(interval)
		}
	}
}

func (acc *adaptiveController) maybeRecalib() {
	if !acc.isWarmedUp() {
		return
	}

	acc.recalibMu.Lock()
	defer acc.recalibMu.Unlock()

	loadGate := atomicLoadFloat64(&acc.recalibLoadGate)
	window := acc.windowSize.Load()
	if float64(acc.inflight.Load()) > loadGate*float64(window) {
		return // Guard 1: under load
	}

	samples := acc.ring.snapshot()
	if len(samples) < 50 {
		return // Guard 2: insufficient data
	}
	candidate := rttP25(samples)
	if candidate <= 0 {
		return
	}

	current := acc.baselineRTT.Load()
	maxIncrease := atomicLoadFloat64(&acc.recalibMaxIncrease)
	if float64(candidate) > float64(current)*maxIncrease {
		slog.Debug("acc: recalibration candidate rejected (probable crisis artifact)",
			"current_baseline_ms", current,
			"candidate_p25_ms", candidate,
		)
		return // Guard 3: sanity cap
	}

	alpha := atomicLoadFloat64(&acc.recalibAlpha)
	next := int64(float64(current)*(1-alpha) + float64(candidate)*alpha)
	if next == current {
		return
	}
	acc.baselineRTT.Store(next) // Guard 4: EMA applied
	slog.Debug("acc: baseline recalibrated",
		"old_ms", current,
		"new_ms", next,
		"candidate_p25_ms", candidate,
	)
}

// ── etcd config watcher ───────────────────────────────────────────────────────

// runConfigWatcher reads the ACC config from etcd on startup and then watches
// for changes, applying them live. It runs in its own goroutine launched by getACC.
//
// Resilience: if etcd is unavailable, the watcher retries with exponential
// backoff (max 5 min). The service continues with compile-time defaults.
func (acc *adaptiveController) runConfigWatcher() {
	backoff := 5 * time.Second
	const maxBackoff = 5 * time.Minute

	for {
		if err := acc.watchConfig(); err != nil {
			slog.Warn("acc: config watcher stopped, will retry",
				"err", err,
				"backoff", backoff,
			)
		}
		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (acc *adaptiveController) watchConfig() error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	// Initial load.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := cli.Get(ctx, accConfigEtcdKey)
	cancel()
	if err == nil && len(resp.Kvs) > 0 {
		var cfg ACCConfig
		if json.Unmarshal(resp.Kvs[0].Value, &cfg) == nil {
			acc.applyConfig(cfg)
		}
	}

	// Watch for changes.
	wctx, wcancel := context.WithCancel(context.Background())
	defer wcancel()
	wch := cli.Watch(wctx, accConfigEtcdKey)
	for {
		wr, ok := <-wch
		if !ok {
			return clientv3.ErrNoAvailableEndpoints
		}
		if wr.Err() != nil {
			return wr.Err()
		}
		for _, ev := range wr.Events {
			if ev.Kv == nil {
				continue
			}
			var cfg ACCConfig
			if json.Unmarshal(ev.Kv.Value, &cfg) != nil {
				slog.Warn("acc: ignoring malformed config update",
					"key", accConfigEtcdKey)
				continue
			}
			acc.applyConfig(cfg)
		}
	}
}

// applyConfig applies non-zero fields from cfg to the running controller.
// Fields that are zero are left unchanged (preserves current tuning).
func (acc *adaptiveController) applyConfig(cfg ACCConfig) {
	if cfg.P1AuthzSize > 0 {
		acc.p1Authz.resize(cfg.P1AuthzSize)
		slog.Info("acc: P1-authz pool resized", "new_size", cfg.P1AuthzSize)
	}
	if cfg.P1PeriodicSize > 0 {
		acc.p1Periodic.resize(cfg.P1PeriodicSize)
		slog.Info("acc: P1-periodic pool resized", "new_size", cfg.P1PeriodicSize)
	}
	if cfg.P1ControlSize > 0 {
		acc.p1Control.resize(cfg.P1ControlSize)
		slog.Info("acc: P1-control pool resized", "new_size", cfg.P1ControlSize)
	}
	if cfg.P2MinWindow > 0 {
		acc.minWindow.Store(cfg.P2MinWindow)
	}
	if cfg.P2MaxWindow > 0 {
		acc.maxWindow.Store(cfg.P2MaxWindow)
	}
	if cfg.AIMDIncreaseThresholdMult > 0 {
		atomicStoreFloat64(&acc.aIMDIncreaseMult, cfg.AIMDIncreaseThresholdMult)
	}
	if cfg.AIMDDecreaseThresholdMult > 0 {
		atomicStoreFloat64(&acc.aIMDDecreaseMult, cfg.AIMDDecreaseThresholdMult)
	}
	if cfg.AIMDDecreaseRate > 0 && cfg.AIMDDecreaseRate < 1 {
		atomicStoreFloat64(&acc.aIMDDecreaseRate, cfg.AIMDDecreaseRate)
	}
	if cfg.RecalibIntervalSec > 0 {
		acc.recalibIntervalSec.Store(cfg.RecalibIntervalSec)
	}
	if cfg.RecalibAlpha > 0 && cfg.RecalibAlpha < 1 {
		atomicStoreFloat64(&acc.recalibAlpha, cfg.RecalibAlpha)
	}
	if cfg.RecalibMaxIncrease > 1 {
		atomicStoreFloat64(&acc.recalibMaxIncrease, cfg.RecalibMaxIncrease)
	}
	if cfg.RecalibLoadGate > 0 && cfg.RecalibLoadGate < 1 {
		atomicStoreFloat64(&acc.recalibLoadGate, cfg.RecalibLoadGate)
	}
}

// ── Observability ─────────────────────────────────────────────────────────────

// ACCStats is a point-in-time snapshot of the ACC state.
type ACCStats struct {
	Warmup          bool    `json:"warmup"`
	BaselineMs      int64   `json:"baseline_rtt_ms"`
	WindowSize      int64   `json:"p2_window"`
	WindowMin       int64   `json:"p2_window_min"`
	WindowMax       int64   `json:"p2_window_max"`
	Inflight        int64   `json:"p2_inflight"`
	P1AuthzCap      int64   `json:"p1_authz_capacity"`
	P1AuthzInUse    int64   `json:"p1_authz_inflight"`
	P1PeriodicCap   int64   `json:"p1_periodic_capacity"`
	P1PeriodicInUse int64   `json:"p1_periodic_inflight"`
	P1ControlCap    int64   `json:"p1_control_capacity"`
	P1ControlInUse  int64   `json:"p1_control_inflight"`
	P2Admitted      int64   `json:"p2_admitted_total"`
	P2Rejected      int64   `json:"p2_rejected_total"`
	P1Admitted      int64   `json:"p1_admitted_total"`
	P1Rejected      int64   `json:"p1_rejected_total"`
	Samples         int64   `json:"samples_total"`
	RecalibAlpha    float64 `json:"recalib_alpha"`
}

func GetACCStats() ACCStats {
	acc := getACC()
	return ACCStats{
		Warmup:          !acc.isWarmedUp(),
		BaselineMs:      acc.baselineRTT.Load(),
		WindowSize:      acc.windowSize.Load(),
		WindowMin:       acc.minWindow.Load(),
		WindowMax:       acc.maxWindow.Load(),
		Inflight:        acc.inflight.Load(),
		P1AuthzCap:      acc.p1Authz.cap(),
		P1AuthzInUse:    acc.p1Authz.inUse(),
		P1PeriodicCap:   acc.p1Periodic.cap(),
		P1PeriodicInUse: acc.p1Periodic.inUse(),
		P1ControlCap:    acc.p1Control.cap(),
		P1ControlInUse:  acc.p1Control.inUse(),
		P2Admitted:      acc.p2Admitted.Load(),
		P2Rejected:      acc.p2Rejected.Load(),
		P1Admitted:      acc.p1Admitted.Load(),
		P1Rejected:      acc.p1Rejected.Load(),
		Samples:         acc.sampleCount.Load(),
		RecalibAlpha:    atomicLoadFloat64(&acc.recalibAlpha),
	}
}

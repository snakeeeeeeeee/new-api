package service

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

type AsyncWorkerRuntimeStats struct {
	Running               bool  `json:"running"`
	Concurrency           int   `json:"concurrency"`
	EndpointConcurrency   int   `json:"endpoint_concurrency,omitempty"`
	InFlight              int   `json:"in_flight"`
	Available             int   `json:"available"`
	Saturated             bool  `json:"saturated"`
	StartedAt             int64 `json:"started_at"`
	AttemptedSinceStart   int64 `json:"attempted_since_start"`
	SucceededSinceStart   int64 `json:"succeeded_since_start"`
	FailedSinceStart      int64 `json:"failed_since_start"`
	TimedOutSinceStart    int64 `json:"timed_out_since_start"`
	AverageDurationMS     int64 `json:"average_duration_ms"`
	RequestTimeoutSeconds int   `json:"request_timeout_seconds"`
}

type workerAttemptResult struct {
	succeeded bool
	timedOut  bool
}

type asyncWorkerRuntime struct {
	mu sync.Mutex

	running          bool
	started          bool
	startedAt        int64
	inFlight         int
	endpointInFlight map[int64]int
	attempted        int64
	succeeded        int64
	failed           int64
	timedOut         int64
	totalDurationMS  int64

	wake     chan struct{}
	stop     chan struct{}
	loopDone chan struct{}
	stopOnce sync.Once
	jobs     sync.WaitGroup
}

func newAsyncWorkerRuntime() *asyncWorkerRuntime {
	return &asyncWorkerRuntime{
		endpointInFlight: make(map[int64]int),
		wake:             make(chan struct{}, 1),
		stop:             make(chan struct{}),
		loopDone:         make(chan struct{}),
	}
}

func (r *asyncWorkerRuntime) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = true
	r.running = true
	if r.startedAt == 0 {
		r.startedAt = time.Now().Unix()
	}
}

func (r *asyncWorkerRuntime) stopLoop() {
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
	close(r.loopDone)
}

func (r *asyncWorkerRuntime) signalStop() {
	r.stopOnce.Do(func() { close(r.stop) })
}

func (r *asyncWorkerRuntime) tryStart(endpointID int64, concurrency int, endpointConcurrency int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if concurrency <= 0 || r.inFlight >= concurrency {
		return false
	}
	if endpointID > 0 && endpointConcurrency > 0 && r.endpointInFlight[endpointID] >= endpointConcurrency {
		return false
	}
	r.inFlight++
	if endpointID > 0 {
		r.endpointInFlight[endpointID]++
	}
	r.attempted++
	r.jobs.Add(1)
	return true
}

func (r *asyncWorkerRuntime) finish(endpointID int64, started time.Time, result workerAttemptResult) {
	durationMS := time.Since(started).Milliseconds()
	r.mu.Lock()
	if r.inFlight > 0 {
		r.inFlight--
	}
	if endpointID > 0 {
		if count := r.endpointInFlight[endpointID]; count <= 1 {
			delete(r.endpointInFlight, endpointID)
		} else {
			r.endpointInFlight[endpointID] = count - 1
		}
	}
	if result.succeeded {
		r.succeeded++
	} else {
		r.failed++
	}
	if result.timedOut {
		r.timedOut++
	}
	r.totalDurationMS += durationMS
	r.mu.Unlock()
	r.jobs.Done()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *asyncWorkerRuntime) capacity(concurrency int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	available := concurrency - r.inFlight
	if available < 0 {
		return 0
	}
	return available
}

func (r *asyncWorkerRuntime) endpointCounts() map[int64]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := make(map[int64]int, len(r.endpointInFlight))
	for endpointID, count := range r.endpointInFlight {
		counts[endpointID] = count
	}
	return counts
}

func (r *asyncWorkerRuntime) snapshot(concurrency int, endpointConcurrency int, timeoutSeconds int) AsyncWorkerRuntimeStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	available := concurrency - r.inFlight
	if available < 0 {
		available = 0
	}
	averageDurationMS := int64(0)
	completed := r.succeeded + r.failed
	if completed > 0 {
		averageDurationMS = r.totalDurationMS / completed
	}
	return AsyncWorkerRuntimeStats{
		Running: r.running, Concurrency: concurrency, EndpointConcurrency: endpointConcurrency,
		InFlight: r.inFlight, Available: available, Saturated: concurrency > 0 && r.inFlight >= concurrency,
		StartedAt: r.startedAt, AttemptedSinceStart: r.attempted, SucceededSinceStart: r.succeeded,
		FailedSinceStart: r.failed, TimedOutSinceStart: r.timedOut, AverageDurationMS: averageDurationMS,
		RequestTimeoutSeconds: timeoutSeconds,
	}
}

func (r *asyncWorkerRuntime) wait(ctx context.Context) error {
	r.mu.Lock()
	started := r.started
	r.mu.Unlock()
	if !started {
		return nil
	}
	select {
	case <-r.loopDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	jobsDone := make(chan struct{})
	go func() {
		r.jobs.Wait()
		close(jobsDone)
	}()
	select {
	case <-jobsDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func SignalStopAsyncWorkers() {
	imageTaskDispatchRuntime.signalStop()
	webhookDeliveryRuntime.signalStop()
}

func WaitForAsyncWorkers(ctx context.Context) error {
	defer webhookTransport.CloseIdleConnections()
	if err := imageTaskDispatchRuntime.wait(ctx); err != nil {
		return err
	}
	return webhookDeliveryRuntime.wait(ctx)
}

func StopAsyncWorkers(ctx context.Context) error {
	SignalStopAsyncWorkers()
	return WaitForAsyncWorkers(ctx)
}

func workerErrorTimedOut(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

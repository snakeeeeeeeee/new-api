package common

import (
	"context"
	"time"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func (info *RelayInfo) ShouldUseOpenAIResponseIntegrity() bool {
	if info == nil || !info.OpenAIResponseIntegrityEnabled {
		return false
	}
	return info.RelayMode == relayconstant.RelayModeChatCompletions ||
		info.RelayMode == relayconstant.RelayModeResponses
}

func (info *RelayInfo) BeginOpenAIResponseIntegrityAttempt(parent context.Context) context.Context {
	if !info.ShouldUseOpenAIResponseIntegrity() {
		return parent
	}
	if parent == nil {
		parent = context.Background()
	}

	info.EndOpenAIResponseIntegrityAttempt()
	ctx, cancel := context.WithCancel(parent)
	info.SendResponseCount = 0
	info.ReceivedResponseCount = 0
	info.FirstResponseTime = info.StartTime.Add(-time.Second)
	info.isFirstResponse = true
	attempt := &openAIResponseIntegrityAttempt{
		ctx:       ctx,
		cancel:    cancel,
		startedAt: time.Now(),
	}
	info.openAIResponseIntegrityAttempt = attempt

	timeout := info.OpenAIResponseIntegrityFirstOutputTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if info.IsStream {
		info.DisablePing = true
	}
	attempt.firstOutputPending.Store(true)
	attempt.firstOutputTimer = time.AfterFunc(timeout, func() {
		if attempt.firstOutputPending.CompareAndSwap(true, false) {
			attempt.firstOutputTimeout.Store(true)
			cancel()
		}
	})
	return ctx
}

func (info *RelayInfo) OpenAIResponseIntegrityAttemptContext() context.Context {
	if info == nil || info.openAIResponseIntegrityAttempt == nil {
		return nil
	}
	return info.openAIResponseIntegrityAttempt.ctx
}

func (info *RelayInfo) OpenAIResponseIntegrityAttemptDone() <-chan struct{} {
	ctx := info.OpenAIResponseIntegrityAttemptContext()
	if ctx == nil {
		return nil
	}
	return ctx.Done()
}

func (info *RelayInfo) OpenAIResponseIntegrityAttemptElapsed() time.Duration {
	if info == nil || info.openAIResponseIntegrityAttempt == nil {
		return 0
	}
	startedAt := info.openAIResponseIntegrityAttempt.startedAt
	if startedAt.IsZero() {
		return 0
	}
	return time.Since(startedAt)
}

func (info *RelayInfo) MarkOpenAIResponseIntegrityFirstOutput() {
	if info == nil || !info.OpenAIResponseIntegrityEnabled {
		return
	}
	attempt := info.openAIResponseIntegrityAttempt
	if attempt == nil || !attempt.firstOutputPending.CompareAndSwap(true, false) {
		return
	}
	attempt.mutex.Lock()
	if attempt.firstOutputTimer != nil {
		attempt.firstOutputTimer.Stop()
	}
	attempt.mutex.Unlock()
}

func (info *RelayInfo) OpenAIResponseIntegrityFirstOutputTimedOut() bool {
	return info != nil && info.openAIResponseIntegrityAttempt != nil &&
		info.openAIResponseIntegrityAttempt.firstOutputTimeout.Load()
}

func (info *RelayInfo) EndOpenAIResponseIntegrityAttempt() {
	if info == nil {
		return
	}
	attempt := info.openAIResponseIntegrityAttempt
	info.openAIResponseIntegrityAttempt = nil
	if attempt == nil {
		return
	}
	attempt.firstOutputPending.Store(false)
	attempt.mutex.Lock()
	timer := attempt.firstOutputTimer
	cancel := attempt.cancel
	attempt.firstOutputTimer = nil
	attempt.cancel = nil
	attempt.mutex.Unlock()
	if timer != nil {
		timer.Stop()
	}
	if cancel != nil {
		cancel()
	}
}

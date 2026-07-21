package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncWorkerRuntimeEnforcesGlobalAndEndpointCapacity(t *testing.T) {
	runtime := newAsyncWorkerRuntime()
	runtime.start()

	require.True(t, runtime.tryStart(101, 4, 2))
	require.True(t, runtime.tryStart(101, 4, 2))
	assert.False(t, runtime.tryStart(101, 4, 2))
	require.True(t, runtime.tryStart(202, 4, 2))
	require.True(t, runtime.tryStart(202, 4, 2))
	assert.False(t, runtime.tryStart(303, 4, 2))

	snapshot := runtime.snapshot(4, 2, 10)
	assert.Equal(t, 4, snapshot.InFlight)
	assert.Zero(t, snapshot.Available)
	assert.True(t, snapshot.Saturated)

	runtime.finish(101, time.Now().Add(-10*time.Millisecond), workerAttemptResult{succeeded: true})
	require.True(t, runtime.tryStart(303, 4, 2))
	assert.False(t, runtime.tryStart(404, 2, 2), "a hot concurrency decrease must block new work until in-flight falls below the new limit")

	for _, endpointID := range []int64{101, 202, 202, 303} {
		runtime.finish(endpointID, time.Now(), workerAttemptResult{succeeded: true})
	}
}

package gognee

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTrace(t *testing.T) {
	trace := newTrace()
	assert.NotNil(t, trace)
	assert.NotNil(t, trace.Spans)
	assert.Equal(t, 0, len(trace.Spans))
	assert.Equal(t, int64(0), trace.TotalDurationMs)
}

func TestTraceAddSpan(t *testing.T) {
	trace := newTrace()
	
	span1 := Span{
		Name:       "test-span-1",
		DurationMs: 100,
		OK:         true,
		Counters:   map[string]int64{"count": 5},
	}
	trace.addSpan(span1)
	
	assert.Equal(t, 1, len(trace.Spans))
	assert.Equal(t, int64(100), trace.TotalDurationMs)
	assert.Equal(t, "test-span-1", trace.Spans[0].Name)
	
	span2 := Span{
		Name:       "test-span-2",
		DurationMs: 50,
		OK:         false,
		Error:      "test error",
	}
	trace.addSpan(span2)
	
	assert.Equal(t, 2, len(trace.Spans))
	assert.Equal(t, int64(150), trace.TotalDurationMs)
	assert.Equal(t, "test error", trace.Spans[1].Error)
}

func TestSpanTimerDisabled(t *testing.T) {
	// When tracing is disabled, span timer should be a no-op
	trace := newTrace()
	timer := newSpanTimer("test", trace, false)
	
	assert.False(t, timer.enabled)
	
	// Finish should not add span
	timer.finish(true, nil, map[string]int64{"count": 1})
	assert.Equal(t, 0, len(trace.Spans))
	assert.Equal(t, int64(0), trace.TotalDurationMs)
}

func TestSpanTimerEnabled(t *testing.T) {
	trace := newTrace()
	timer := newSpanTimer("test-operation", trace, true)
	
	assert.True(t, timer.enabled)
	assert.Equal(t, "test-operation", timer.name)
	
	// Simulate some work
	time.Sleep(10 * time.Millisecond)
	
	counters := map[string]int64{"items": 42}
	timer.finish(true, nil, counters)
	
	assert.Equal(t, 1, len(trace.Spans))
	assert.Equal(t, "test-operation", trace.Spans[0].Name)
	assert.True(t, trace.Spans[0].OK)
	assert.GreaterOrEqual(t, trace.Spans[0].DurationMs, int64(10))
	assert.Equal(t, int64(42), trace.Spans[0].Counters["items"])
	assert.Equal(t, "", trace.Spans[0].Error)
}

func TestSpanTimerWithError(t *testing.T) {
	trace := newTrace()
	timer := newSpanTimer("failing-operation", trace, true)
	
	testErr := assert.AnError
	timer.finish(false, testErr, nil)
	
	assert.Equal(t, 1, len(trace.Spans))
	assert.False(t, trace.Spans[0].OK)
	assert.Equal(t, testErr.Error(), trace.Spans[0].Error)
}

func TestSpanTimerNilTrace(t *testing.T) {
	// Should not panic when trace is nil
	timer := newSpanTimer("test", nil, true)
	assert.False(t, timer.enabled)
	
	timer.finish(true, nil, nil)
	// Should not panic
}

func TestTraceOverheadNegligible(t *testing.T) {
	// Benchmark: overhead should be <1ms when disabled
	iterations := 1000
	
	trace := newTrace()
	start := time.Now()
	for i := 0; i < iterations; i++ {
		timer := newSpanTimer("test", trace, false)
		timer.finish(true, nil, map[string]int64{"count": 1})
	}
	elapsed := time.Since(start)
	
	// 1000 no-op timers should take <1ms total
	assert.Less(t, elapsed.Milliseconds(), int64(1))
	assert.Equal(t, 0, len(trace.Spans))
}

func TestTimeNowMs(t *testing.T) {
	// Test that timeNowMs returns reasonable values
	before := time.Now().UnixMilli()
	actual := timeNowMs()
	after := time.Now().UnixMilli()
	
	assert.GreaterOrEqual(t, actual, before)
	assert.LessOrEqual(t, actual, after)
}

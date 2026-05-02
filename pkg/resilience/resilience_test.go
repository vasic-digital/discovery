package resilience

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testLogger is a no-op Logger used by tests.
type testLogger struct {
	messages []string
}

func newTestLogger() *testLogger { return &testLogger{} }

func (l *testLogger) Info(msg string, kv ...interface{})  { l.messages = append(l.messages, "INFO: "+msg) }
func (l *testLogger) Warn(msg string, kv ...interface{})  { l.messages = append(l.messages, "WARN: "+msg) }
func (l *testLogger) Error(msg string, kv ...interface{}) { l.messages = append(l.messages, "ERROR: "+msg) }
func (l *testLogger) Debug(msg string, kv ...interface{}) { l.messages = append(l.messages, "DEBUG: "+msg) }

// testMetrics records SetSourceHealth calls.
type testMetrics struct {
	healthValues map[string]float64
}

func newTestMetrics() *testMetrics {
	return &testMetrics{healthValues: make(map[string]float64)}
}

func (m *testMetrics) SetSourceHealth(sourceID string, value float64) {
	m.healthValues[sourceID] = value
}

// ---------------------------------------------------------------------------
// ConnectionState tests
// ---------------------------------------------------------------------------

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{Connected, "connected"},
		{Disconnected, "disconnected"},
		{Reconnecting, "reconnecting"},
		{Offline, "offline"},
		{ConnectionState(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestConnectionState_HealthMetric(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected float64
	}{
		{Connected, Healthy},
		{Disconnected, Degraded},
		{Reconnecting, Degraded},
		{Offline, OfflineHealth},
		{ConnectionState(99), OfflineHealth},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.HealthMetric())
		})
	}
}

func TestHealthMetricConstants(t *testing.T) {
	assert.Equal(t, 1.0, Healthy)
	assert.Equal(t, 0.5, Degraded)
	assert.Equal(t, 0.0, OfflineHealth)
}

// ---------------------------------------------------------------------------
// EventType tests
// ---------------------------------------------------------------------------

func TestEventType_String(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventConnected, "connected"},
		{EventDisconnected, "disconnected"},
		{EventReconnecting, "reconnecting"},
		{EventOffline, "offline"},
		{EventError, "error"},
		{EventHealthCheck, "health_check"},
		{EventType(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

// ---------------------------------------------------------------------------
// Event tests
// ---------------------------------------------------------------------------

func TestNewEvent(t *testing.T) {
	before := time.Now()
	evt := NewEvent(EventConnected, "src-1", nil)
	after := time.Now()

	require.NotNil(t, evt)
	assert.Equal(t, EventConnected, evt.Type)
	assert.Equal(t, "src-1", evt.SourceID)
	assert.Nil(t, evt.Error)
	assert.True(t, !evt.Timestamp.Before(before) && !evt.Timestamp.After(after))
	assert.Nil(t, evt.Data)
}

func TestEvent_DataField(t *testing.T) {
	evt := NewEvent(EventHealthCheck, "src-3", nil)
	evt.Data = map[string]interface{}{
		"latency_ms": 42,
		"healthy":    true,
	}

	assert.Equal(t, 42, evt.Data["latency_ms"])
	assert.Equal(t, true, evt.Data["healthy"])
}

// ---------------------------------------------------------------------------
// Source tests
// ---------------------------------------------------------------------------

func TestDefaultSource(t *testing.T) {
	src := DefaultSource("nas-1", "NAS Primary", "smb://192.168.1.10")

	require.NotNil(t, src)
	assert.Equal(t, "nas-1", src.ID)
	assert.Equal(t, "NAS Primary", src.Name)
	assert.Equal(t, "smb://192.168.1.10", src.Endpoint)
	assert.Equal(t, Disconnected, src.State)
	assert.Equal(t, 5, src.MaxRetryAttempts)
	assert.Equal(t, 5*time.Second, src.RetryDelay)
	assert.Equal(t, 10*time.Second, src.ConnectionTimeout)
	assert.Equal(t, 30*time.Second, src.HealthCheckInterval)
	assert.True(t, src.IsEnabled)
	assert.Zero(t, src.RetryAttempts)
	assert.True(t, src.LastConnected.IsZero())
	assert.Nil(t, src.LastError)
}

func TestSource_MutexLockUnlock(t *testing.T) {
	src := DefaultSource("test", "Test", "tcp://localhost")

	// Should not panic or deadlock.
	src.Lock()
	src.State = Connected
	src.Unlock()

	assert.Equal(t, Connected, src.State)
}

func TestSource_ZeroValue(t *testing.T) {
	var src Source

	assert.Empty(t, src.ID)
	assert.Empty(t, src.Name)
	assert.Empty(t, src.Endpoint)
	assert.Equal(t, Connected, src.State) // zero value of int = 0 = Connected
	assert.False(t, src.IsEnabled)
}

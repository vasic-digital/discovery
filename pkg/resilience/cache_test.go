package resilience

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// NewOfflineCache tests
// ---------------------------------------------------------------------------

func TestNewOfflineCache(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	require.NotNil(t, cache)
	assert.Equal(t, 0, cache.Size())
	assert.Equal(t, 100, cache.MaxSize())
	assert.False(t, cache.IsOffline())
}

func TestNewOfflineCache_DefaultSize(t *testing.T) {
	cache := NewOfflineCache(0, newTestLogger())
	assert.Equal(t, 1000, cache.MaxSize())
}

func TestNewOfflineCache_NegativeSize(t *testing.T) {
	cache := NewOfflineCache(-5, newTestLogger())
	assert.Equal(t, 1000, cache.MaxSize())
}

// ---------------------------------------------------------------------------
// CacheChange tests
// ---------------------------------------------------------------------------

func TestOfflineCache_CacheChange(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	err := cache.CacheChange("file:updated", "src-1", map[string]string{"path": "/data/file.txt"})
	assert.NoError(t, err)
	assert.Equal(t, 1, cache.Size())
}

func TestOfflineCache_CacheChange_EmptyKey(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	err := cache.CacheChange("", "src-1", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key must not be empty")
	assert.Equal(t, 0, cache.Size())
}

func TestOfflineCache_CacheChange_EmptySourceID(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	err := cache.CacheChange("key", "", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source ID must not be empty")
	assert.Equal(t, 0, cache.Size())
}

func TestOfflineCache_CacheChange_MultipleEntries(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		err := cache.CacheChange(key, "src-1", i)
		require.NoError(t, err)
	}

	assert.Equal(t, 10, cache.Size())
}

func TestOfflineCache_CacheChange_Eviction(t *testing.T) {
	cache := NewOfflineCache(3, newTestLogger())

	require.NoError(t, cache.CacheChange("k1", "s1", "v1"))
	require.NoError(t, cache.CacheChange("k2", "s1", "v2"))
	require.NoError(t, cache.CacheChange("k3", "s1", "v3"))
	assert.Equal(t, 3, cache.Size())

	// Adding a 4th entry should evict the oldest (k1).
	require.NoError(t, cache.CacheChange("k4", "s1", "v4"))
	assert.Equal(t, 3, cache.Size())

	entries := cache.Entries()
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	assert.NotContains(t, keys, "k1")
	assert.Contains(t, keys, "k2")
	assert.Contains(t, keys, "k3")
	assert.Contains(t, keys, "k4")
}

func TestOfflineCache_CacheChange_NilValue(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	err := cache.CacheChange("k1", "s1", nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, cache.Size())

	entries := cache.Entries()
	assert.Nil(t, entries[0].Value)
}

// ---------------------------------------------------------------------------
// ProcessCachedChanges tests
// ---------------------------------------------------------------------------

func TestOfflineCache_ProcessCachedChanges(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	require.NoError(t, cache.CacheChange("k1", "src-a", "v1"))
	require.NoError(t, cache.CacheChange("k2", "src-b", "v2"))
	require.NoError(t, cache.CacheChange("k3", "src-a", "v3"))

	results := cache.ProcessCachedChanges("src-a")

	assert.Len(t, results, 2)
	assert.Equal(t, "k1", results[0].Key)
	assert.Equal(t, "k3", results[1].Key)

	// Only src-b's entry should remain.
	assert.Equal(t, 1, cache.Size())
	remaining := cache.Entries()
	assert.Equal(t, "src-b", remaining[0].SourceID)
}

func TestOfflineCache_ProcessCachedChanges_NoMatch(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	require.NoError(t, cache.CacheChange("k1", "src-a", "v1"))

	results := cache.ProcessCachedChanges("src-nonexistent")
	assert.Empty(t, results)
	assert.Equal(t, 1, cache.Size())
}

func TestOfflineCache_ProcessCachedChanges_EmptyCache(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	results := cache.ProcessCachedChanges("anything")
	assert.Empty(t, results)
}

func TestOfflineCache_ProcessCachedChanges_AllForSource(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	require.NoError(t, cache.CacheChange("k1", "src-a", "v1"))
	require.NoError(t, cache.CacheChange("k2", "src-a", "v2"))
	require.NoError(t, cache.CacheChange("k3", "src-a", "v3"))

	results := cache.ProcessCachedChanges("src-a")
	assert.Len(t, results, 3)
	assert.Equal(t, 0, cache.Size())
}

func TestOfflineCache_ProcessCachedChanges_PreservesOrder(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		require.NoError(t, cache.CacheChange(key, "ordered", i))
	}

	results := cache.ProcessCachedChanges("ordered")
	require.Len(t, results, 5)

	for i, entry := range results {
		assert.Equal(t, fmt.Sprintf("key-%d", i), entry.Key)
		assert.Equal(t, i, entry.Value)
	}
}

// ---------------------------------------------------------------------------
// EnableOfflineMode / DisableOfflineMode tests
// ---------------------------------------------------------------------------

func TestOfflineCache_OfflineMode(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	assert.False(t, cache.IsOffline())

	cache.EnableOfflineMode()
	assert.True(t, cache.IsOffline())

	cache.DisableOfflineMode()
	assert.False(t, cache.IsOffline())
}

func TestOfflineCache_OfflineMode_Idempotent(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	cache.EnableOfflineMode()
	cache.EnableOfflineMode()
	assert.True(t, cache.IsOffline())

	cache.DisableOfflineMode()
	cache.DisableOfflineMode()
	assert.False(t, cache.IsOffline())
}

// ---------------------------------------------------------------------------
// Clear tests
// ---------------------------------------------------------------------------

func TestOfflineCache_Clear(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	for i := 0; i < 5; i++ {
		require.NoError(t, cache.CacheChange(fmt.Sprintf("k%d", i), "src", i))
	}
	assert.Equal(t, 5, cache.Size())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())
	assert.Empty(t, cache.Entries())
}

func TestOfflineCache_Clear_Empty(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

// ---------------------------------------------------------------------------
// Entries tests
// ---------------------------------------------------------------------------

func TestOfflineCache_Entries_ReturnsCopy(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	require.NoError(t, cache.CacheChange("k1", "src", "v1"))

	entries := cache.Entries()
	assert.Len(t, entries, 1)

	// Modifying the returned slice should not affect the cache.
	entries[0] = nil
	assert.Equal(t, 1, cache.Size())

	entries2 := cache.Entries()
	assert.NotNil(t, entries2[0])
}

func TestOfflineCache_Entries_Empty(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	entries := cache.Entries()
	assert.Empty(t, entries)
}

// ---------------------------------------------------------------------------
// EntriesForSource tests
// ---------------------------------------------------------------------------

func TestOfflineCache_EntriesForSource(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	require.NoError(t, cache.CacheChange("k1", "src-a", "v1"))
	require.NoError(t, cache.CacheChange("k2", "src-b", "v2"))
	require.NoError(t, cache.CacheChange("k3", "src-a", "v3"))

	entries := cache.EntriesForSource("src-a")
	assert.Len(t, entries, 2)
	assert.Equal(t, "k1", entries[0].Key)
	assert.Equal(t, "k3", entries[1].Key)

	// Should not remove entries.
	assert.Equal(t, 3, cache.Size())
}

func TestOfflineCache_EntriesForSource_NoMatch(t *testing.T) {
	cache := NewOfflineCache(100, newTestLogger())

	require.NoError(t, cache.CacheChange("k1", "src-a", "v1"))

	entries := cache.EntriesForSource("src-missing")
	assert.Empty(t, entries)
}

// ---------------------------------------------------------------------------
// CacheEntry tests
// ---------------------------------------------------------------------------

func TestCacheEntry_Fields(t *testing.T) {
	cache := NewOfflineCache(10, newTestLogger())

	require.NoError(t, cache.CacheChange("file:created", "nas-1", map[string]string{
		"path": "/share/file.mp4",
		"size": "1024",
	}))

	entries := cache.Entries()
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, "file:created", entry.Key)
	assert.Equal(t, "nas-1", entry.SourceID)
	assert.False(t, entry.CachedAt.IsZero())

	val, ok := entry.Value.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "/share/file.mp4", val["path"])
}

// ---------------------------------------------------------------------------
// Concurrent access tests
// ---------------------------------------------------------------------------

func TestOfflineCache_ConcurrentCacheChange(t *testing.T) {
	cache := NewOfflineCache(1000, newTestLogger())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			_ = cache.CacheChange(key, "src", idx)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 50, cache.Size())
}

func TestOfflineCache_ConcurrentReadWrite(t *testing.T) {
	// bluff-scan: no-assert-ok (concurrency test — go test -race catches data races; absence of panic == correctness)
	cache := NewOfflineCache(100, newTestLogger())

	var wg sync.WaitGroup

	// Writers.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = cache.CacheChange(fmt.Sprintf("k-%d", idx), "src", idx)
		}(i)
	}

	// Readers.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cache.Size()
			_ = cache.Entries()
			_ = cache.IsOffline()
			_ = cache.EntriesForSource("src")
		}()
	}

	wg.Wait()
	// No race conditions or panics.
}

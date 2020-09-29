package distrox

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSyncMapCacheWriteAndGet(t *testing.T) {
	t.Parallel()

	c, err := newSyncMapCache(10 * time.Minute)
	assert.Nil(t, err)

	defer c.reset()
	defer c.close()

	iterationCount := 100

	wantCacheBytes := 0
	for i := 0; i < iterationCount; i++ {
		key := []byte(fmt.Sprintf("key %d", i))
		want := []byte(fmt.Sprintf("value %d", i))

		err := c.setBin(key, want)
		assert.Nil(t, err)
		wantCacheBytes += len(want) // + timestampSizeInBytes

		got, err := c.getBin(key)
		assert.Nil(t, err)
		assert.Equal(t, string(got), string(want))
	}

	var stats CacheStats
	c.loadStats(&stats)
	want := uint64(iterationCount)
	assert.Equal(t, want, stats.EntriesCount)
	assert.Equal(t, wantCacheBytes, int(stats.CacheBytes))

	assert.Equal(t, want, stats.Hits)
	assert.Equal(t, want, stats.EntriesCount)

	assert.Empty(t, stats.DelHits)
	assert.Empty(t, stats.Misses)
	assert.Empty(t, stats.DelMisses)
	assert.Empty(t, stats.DelMisses)
}

func TestSyncMapCacheExpireItems(t *testing.T) {
	t.Parallel()

	var err error
	c, err := newSyncMapCache(3 * time.Second)
	assert.Nil(t, err)

	defer c.reset()
	defer c.close()

	iterationCount := 100
	for i := 0; i < iterationCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		want := []byte(fmt.Sprintf("value %d", i))
		err := c.setBin(k, want)
		assert.Nil(t, err)
	}

	time.Sleep(5 * time.Second)

	for i := 0; i < iterationCount; i++ {
		key := []byte(fmt.Sprintf("key %d", i))

		got, err := c.getBin(key)
		assert.Nil(t, got)
		assert.Equal(t, err, ErrEntryNotFound)
	}

	var stats CacheStats
	c.loadStats(&stats)
	want := uint64(iterationCount)
	assert.Equal(t, want, stats.Misses)

	assert.Empty(t, stats.EntriesCount)
	assert.Empty(t, stats.CacheBytes)

	assert.Empty(t, stats.Hits)
	assert.Empty(t, stats.DelHits)
	assert.Empty(t, stats.DelMisses)
}

func TestSyncMapCacheDel(t *testing.T) {
	t.Parallel()

	c, err := newSyncMapCache(10 * time.Minute)
	assert.Nil(t, err)

	defer c.reset()
	defer c.close()

	iterationCount := 100
	for i := 0; i < iterationCount; i++ {
		key := []byte(fmt.Sprintf("key %d", i))
		want := []byte(fmt.Sprintf("value %d", i))
		err := c.setBin(key, want)
		assert.Nil(t, err)

		got, err := c.getBin(key)
		assert.Nil(t, err)

		assert.Equal(t, string(got), string(want))

		err = c.delBin(key)
		assert.Nil(t, err)

		got, err = c.getBin(key)
		assert.Nil(t, got)
		assert.Equal(t, err, ErrEntryNotFound)
	}

	var stats CacheStats
	c.loadStats(&stats)
	want := uint64(iterationCount)
	assert.Equal(t, want, stats.Hits)
	assert.Equal(t, want, stats.DelHits)
	assert.Equal(t, want, stats.Misses)

	assert.Empty(t, stats.EntriesCount)
	assert.Empty(t, stats.DelMisses)
	assert.Empty(t, stats.CacheBytes)
}

func TestSyncMapCacheGetSetConcurrently(t *testing.T) {
	itemsCount := 10000
	const goroutines = 20

	c, err := newSyncMapCache(10 * time.Minute)
	assert.Nil(t, err)

	defer c.reset()
	defer c.close()

	ch := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			ch <- assertSyncMapCacheGetSet(t, c, itemsCount)
		}()
	}
	for i := 0; i < goroutines; i++ {
		select {
		case err := <-ch:
			assert.Nil(t, err)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func assertSyncMapCacheGetSet(t *testing.T, c *syncMapCache, itemsCount int) error {
	for i := 0; i < itemsCount; i++ {
		key := []byte(fmt.Sprintf("key %d", i))
		want := []byte(fmt.Sprintf("value %d", i))

		err := c.setBin(key, want)
		assert.Nil(t, err)

		got, err := c.getBin(key)
		assert.Nil(t, err)
		assert.Equal(t, string(got), string(want))
	}

	misses := 0
	for i := 0; i < itemsCount; i++ {
		key := []byte(fmt.Sprintf("key %d", i))
		want := fmt.Sprintf("value %d", i)

		got, err := c.getBin(key)
		assert.Nil(t, err)

		if string(got) != want {
			if len(got) > 0 {
				return fmt.Errorf("unexpected value — key: %q, got: %q want: %q", key, got, want)
			}
			misses++
		}
	}
	if misses >= itemsCount/100 {
		return fmt.Errorf("cache misses %d should be less than %d", misses, itemsCount/100)
	}
	return nil
}

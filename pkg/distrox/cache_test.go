package distrox

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheWriteAndGet(t *testing.T) {
	t.Parallel()

	c, err := NewCache(
		WithMaxBytes(1024 * 1024 * 1024),
	)
	assert.Nil(t, err)

	defer c.Reset()
	defer c.Close()

	iterationCount := 100

	for i := 0; i < iterationCount; i++ {
		key := fmt.Sprintf("key %d", i)
		want := []byte(fmt.Sprintf("value %d", i))

		err := c.Set(key, want)
		assert.Nil(t, err)

		got, err := c.Get(key)
		assert.Nil(t, err)
		assert.Equal(t, string(got), string(want))
	}

	var stats CacheStats
	c.LoadStats(&stats)
	want := uint64(iterationCount)
	assert.Equal(t, want, stats.Hits)
	assert.Equal(t, want, stats.EntriesCount)

	assert.Empty(t, stats.DelHits)
	assert.Empty(t, stats.Misses)
	assert.Empty(t, stats.DelMisses)
	assert.Empty(t, stats.DelMisses)
	assert.Empty(t, stats.Collisions)
}

func TestCacheExpireItems(t *testing.T) {
	t.Parallel()

	var err error
	c, err := NewCache(
		WithMaxBytes(1024*1024*1024),
		WithTTLDuration(3*time.Second),
	)
	assert.Nil(t, err)

	defer c.Reset()
	defer c.Close()

	iterationCount := 100
	for i := 0; i < iterationCount; i++ {
		key := fmt.Sprintf("key %d", i)
		want := []byte(fmt.Sprintf("value %d", i))
		err := c.Set(key, want)
		assert.Nil(t, err)
	}

	time.Sleep(6 * time.Second)

	for i := 0; i < iterationCount; i++ {
		k := fmt.Sprintf("key %d", i)

		got, err := c.Get(k)
		assert.Nil(t, got)
		assert.Equal(t, err, ErrEntryNotFound)
	}

	var stats CacheStats
	c.LoadStats(&stats)
	want := uint64(iterationCount)
	assert.Equal(t, want, stats.Misses)

	assert.Empty(t, stats.Hits)
	assert.Empty(t, stats.EntriesCount)
	assert.Empty(t, stats.DelHits)
	assert.Empty(t, stats.DelMisses)
	assert.Empty(t, stats.Collisions)
}

func TestCacheDel(t *testing.T) {
	t.Parallel()

	c, err := NewCache(
		WithMaxBytes(1024 * 1024 * 1024),
	)
	assert.Nil(t, err)

	defer c.Reset()
	defer c.Close()

	iterationCount := 100
	for i := 0; i < iterationCount; i++ {
		key := fmt.Sprintf("key %d", i)
		want := []byte(fmt.Sprintf("value %d", i))
		err := c.Set(key, want)
		assert.Nil(t, err)

		got, err := c.Get(key)
		assert.Nil(t, err)

		assert.Equal(t, string(got), string(want))

		err = c.Del(key)
		assert.Nil(t, err)

		got, err = c.Get(key)
		assert.Nil(t, got)
		assert.Equal(t, err, ErrEntryNotFound)
	}

	var stats CacheStats
	c.LoadStats(&stats)
	want := uint64(iterationCount)
	assert.Equal(t, want, stats.Hits)
	assert.Equal(t, want, stats.DelHits)
	assert.Equal(t, want, stats.Misses)

	assert.Empty(t, stats.EntriesCount)
	assert.Empty(t, stats.DelMisses)
	assert.Empty(t, stats.Collisions)
}

func TestCacheGetSetConcurrently(t *testing.T) {
	itemsCount := 10000
	const goroutines = 20

	c, err := NewCache(
		WithMaxBytes(30 * itemsCount * goroutines),
	)
	assert.Nil(t, err)

	defer c.Reset()
	defer c.Close()

	ch := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			ch <- assertGetSet(t, c, itemsCount)
		}()
	}
	for i := 0; i < goroutines; i++ {
		select {
		case err := <-ch:
			assert.Nil(t, err)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out")
		}
	}
}

func TestCacheSetGetFragmented(t *testing.T) {
	c, err := NewCache(WithMaxBytes(256 * 1024 * 1024))
	assert.Nil(t, err)
	defer c.Reset()
	defer c.Close()

	const valuesCount = 10
	// 524288 max value size
	for _, valueBytes := range []int{1, 100, 65535, 65536, 65537, 131072, 131073, 131071, 524288} {
		t.Run(fmt.Sprintf("Value bytes: %d", valueBytes), func(t *testing.T) {
			for seed := 0; seed < 3; seed++ {
				assertSetGetFragmented(t, c, valueBytes, valuesCount, seed)
			}
		})
	}
}

func assertSetGetFragmented(t *testing.T, c *Cache, valueSize, valuesCount, seed int) {
	m := make(map[string][]byte)
	var buf []byte
	var err error
	for i := 0; i < valuesCount; i++ {
		key := []byte(fmt.Sprintf("key %d", i))
		value := createValue(valueSize, seed)
		err = c.SetBin(key, value)
		assert.Nil(t, err)

		m[string(key)] = value
		buf, err = c.GetBin(buf[:0], key)
		assert.Nil(t, err)
		if !bytes.Equal(buf, value) {
			t.Fatalf("seed:%d — unexpected value — key:%q, got:%d, want:%d",
				seed, key, len(buf), len(value))
		}
	}
	var s CacheStats
	c.LoadStats(&s)

	// Verify that values still exist
	for key, value := range m {
		buf, err = c.GetBin(buf[:0], []byte(key))
		assert.Nil(t, err)
		if !bytes.Equal(buf, value) {
			t.Fatalf(
				"seed:%d — unexpected value received key:%q, got:%d, want:%d",
				seed, key, len(buf), len(value))
		}
	}
}

func assertGetSet(t *testing.T, c *Cache, itemsCount int) error {
	for i := 0; i < itemsCount; i++ {
		k := fmt.Sprintf("key %d", i)
		want := []byte(fmt.Sprintf("value %d", i))

		err := c.Set(k, want)
		assert.Nil(t, err)

		got, err := c.Get(k)
		assert.Nil(t, err)
		assert.Equal(t, string(got), string(want))
	}

	misses := 0
	for i := 0; i < itemsCount; i++ {
		k := fmt.Sprintf("key %d", i)
		want := fmt.Sprintf("value %d", i)

		got, err := c.Get(k)
		assert.Nil(t, err)

		if string(got) != want {
			if len(got) > 0 {
				return fmt.Errorf("unexpected value — key: %q, got: %q want: %q", k, got, want)
			}
			misses++
		}
	}
	if misses >= itemsCount/100 {
		return fmt.Errorf("cache misses %d should be less than %d", misses, itemsCount/100)
	}
	return nil
}

func createValue(size, seed int) []byte {
	var buf []byte
	for i := 0; i < size; i++ {
		buf = append(buf, byte(i+seed))
	}
	return buf
}

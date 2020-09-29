package distrox

import (
	"fmt"
	"time"

	"github.com/ziyasal/distroxy/internal/pkg/common"
)

// WithShards number of cache shards, value must be a power of two
func WithShards(count int) cacheOption {
	return func(c *Cache) error {
		if !isPowerOfTwo(count) {
			return fmt.Errorf("shard count must be power of two")
		}

		c.shardCount = count
		return nil
	}
}

// WithTTL sets global ttlInSeconds for cache
func WithTTL(ttl int64) cacheOption {
	return func(c *Cache) error {
		c.ttlInSeconds = ttl
		return nil
	}
}
func WithTTLDuration(ttl time.Duration) cacheOption {
	return func(c *Cache) error {
		c.ttlInSeconds = int64(ttl.Seconds())
		return nil
	}
}

// WithMaxBytes sets a limit for cache size. Cache will not allocate more
// memory than this limit. It can protect application from consuming all
// available memory on the machine, therefore from running OOM Killer.
//
// When the limit is reached then the oldest entries are overridden
// by the new ones.
func WithMaxBytes(size int) cacheOption {
	return func(c *Cache) error {
		c.maxCacheBytes = size
		return nil
	}
}

// WithHasher sets hasher, by default xxh3 hashing is used.
func WithHasher(h common.Hasher) cacheOption {
	return func(c *Cache) error {
		c.hash = h
		return nil
	}
}

func WithLogger(l common.Logger) cacheOption {
	return func(c *Cache) error {
		c.logger = l
		return nil
	}
}

func WithStatsEnabled() cacheOption {
	return func(c *Cache) error {
		c.statsEnabled = true
		return nil
	}
}

func WithClock(klock common.StoppableClock) cacheOption {
	return func(c *Cache) error {
		c.clock = klock
		return nil
	}
}

func WithMaxKeySize(size int64) cacheOption {
	return func(c *Cache) error {
		c.MaxKeySizeInBytes = size
		return nil
	}
}
func WithMaxValueSize(size int64) cacheOption {
	return func(c *Cache) error {
		c.MaxValueSizeInBytes = size
		return nil
	}
}

func isPowerOfTwo(number int) bool {
	return (number & (number - 1)) == 0
}

package distrox

import (
	"errors"
	"fmt"
	"time"

	"github.com/ziyasal/distroxy/internal/pkg/common"
)

const (
	defaultTTL        = int64(30 * time.Minute)
	defaultShardCount = 512
	maxCacheBytes     = 32 * 1024 * 1024

	maxShardSizeInBytes        = 1073741824 // 1 GB
	defaultMemBlockSizeInBytes = 64 * 1024

	fragmentedEntryKeyLen = 16 // value hash + fragmentIdx
)

var (
	ErrEntryNotFound = errors.New("entry not found")
)

type cacheOption func(cache *Cache) error

// Cache defines a struct to hold kv entries
type Cache struct {
	shardCount int
	shards     []*shard
	shardMask  uint64

	clock  common.StoppableClock
	hash   common.Hasher
	logger common.Logger

	ttlInSeconds int64

	maxCacheBytes int

	statsEnabled bool

	MaxKeySizeInBytes   int64
	MaxValueSizeInBytes int64

	// it's used while processing entries where don't fit into default mem-block
	bpool common.Pooled
}

// NewCache initialize new instance of Cache
func NewCache(opts ...cacheOption) (*Cache, error) {
	c := &Cache{
		shardCount:    defaultShardCount,
		maxCacheBytes: maxCacheBytes,
		clock:         common.NewCachedClock(),
		logger:        common.NewDefaultLogger(),
		hash:          common.NewDefaultHasher(),
		ttlInSeconds:  defaultTTL,
		statsEnabled:  true,

		MaxKeySizeInBytes:   defaultKeySizeInBytes,
		MaxValueSizeInBytes: defaultValueSizeInBytes,
		bpool:               common.NewDefaultPooled(0),
	}

	// apply options
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, errors.New("cache could not created")
		}
	}

	// initialize shard related fields
	err := c.initShards()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Get reads entry for the key and it returns an ErrEntryNotFound when
// no entry exists for the given key.
func (c *Cache) Get(k string) ([]byte, error) {
	return c.GetBin(nil, []byte(k))
}

// Set saves entry under the key
func (c *Cache) Set(k string, v []byte) error {
	return c.SetBin([]byte(k), v)
}

func (c *Cache) SetBin(key []byte, entry []byte) error {
	if len(entry) > defaultValueSizeInBytes {
		return c.setFragmented(key, entry)
	}

	return c.setBin(key, entry, false)
}

// GetBin gets an entry with byte array key,
// if retBuf is passed entry value can be filled to it
func (c *Cache) GetBin(retBuf []byte, key []byte) ([]byte, error) {
	retBuf, isBigEntry, err := c.getBin(retBuf, key)

	if err != nil {
		return nil, err
	}

	if isBigEntry {
		//pass retBuf nil here because it has metadata value to be processed
		return c.getFragmented(nil, retBuf)
	}

	return retBuf, nil
}

// CacheStats returns cache's statistics
func (c *Cache) LoadStats(stats *CacheStats) {
	for _, shard := range c.shards {
		shard.loadStats(stats)
	}
}

// Del removes the key
func (c *Cache) Del(key string) error {
	hashedKey := c.hash.HashStr(key)
	return c.shards[hashedKey&c.shardMask].del(hashedKey)
}

// Del removes the key
func (c *Cache) DelBin(key []byte) error {
	hashedKey := c.hash.Hash(key)
	return c.shards[hashedKey&c.shardMask].del(hashedKey)
}

// Reset empties all cache shards
func (c *Cache) Reset() error {
	for i := range c.shards {
		// return error from shard
		c.shards[i].reset()
	}

	return nil
}

// Len computes number of entries in cache
func (c *Cache) Len() uint64 {
	var length uint64
	for _, shard := range c.shards {
		length += shard.len()
	}
	return length
}

// Close is used to signal a shutdown of the cache to ensure cleanup
func (c *Cache) Close() error {
	c.clock.Stop()

	return nil
}

// initShards initializes shards with computed values
func (c *Cache) initShards() error {
	c.shards = make([]*shard, c.shardCount)
	c.shardMask = uint64(c.shardCount - 1)

	maxBytes := c.maximumShardSizeInBytes()

	for i := 0; i < c.shardCount; i++ {
		s, err := newShard(maxBytes,
			defaultMemBlockSizeInBytes,
			c.ttlInSeconds,
			uint64(maxShardSizeInBytes),
			c.clock,
			c.logger,
			c.statsEnabled)

		if err != nil {
			return err
		}

		c.shards[i] = s
	}

	return nil
}

// setBin private method with more parameters to be used
// while storing non-fragmented and fragmented entries
func (c *Cache) setBin(key []byte, entry []byte, isFragmentedEntry bool) error {
	hashedKey := c.hash.Hash(key)
	s := c.shards[hashedKey&c.shardMask]

	return s.set(key, entry, hashedKey, isFragmentedEntry)
}

// setBin private method with more parameters to be used
// while getting non-fragmented and fragmented entries
func (c *Cache) getBin(retBuf []byte, key []byte) ([]byte, bool, error) {
	hashedKey := c.hash.Hash(key)
	s := c.shards[hashedKey&c.shardMask]

	retBuf, isFragmentedEntry, err := s.get(retBuf, key, hashedKey, true)

	if err != nil {
		return nil, false, err
	}

	return retBuf, isFragmentedEntry, nil
}

func (c *Cache) setFragmented(k []byte, v []byte) error {
	if len(k) > defaultKeySizeInBytes {
		//atomic.AddUint64(&c.bigStats.TooBigKeyErrors, 1)
		return errors.New("too big key")
	}
	valueLen := len(v)
	valueHash := c.hash.Hash(v)

	// Split v into fragments with up to default-value-size each.
	fragmentBuf := c.bpool.Get()
	defer c.bpool.Put(fragmentBuf)

	var i uint64
	for len(v) > 0 {
		fragmentBuf = common.MarshalUint64(fragmentBuf[:0], valueHash)
		fragmentBuf = common.MarshalUint64(fragmentBuf, i)
		i++
		fragmentLen := defaultValueSizeInBytes - 1
		if len(v) < fragmentLen {
			fragmentLen = len(v)
		}
		fragment := v[:fragmentLen]
		v = v[fragmentLen:]

		// set as non fragmented - only metadata entry will have this flag set with true
		err := c.setBin(fragmentBuf, fragment, false)
		if err != nil {
			return err
		}
	}

	// write metadata value, which consists of value hash and value len.
	fragmentBuf = common.MarshalUint64(fragmentBuf[:0], valueHash)
	fragmentBuf = common.MarshalUint64(fragmentBuf, uint64(valueLen))

	// set as fragmented - the (meta) entry value consists of value hash and value len
	// and fragmented entry flag is set to true.
	// Value of this entry will be processed to collect fragments of the actual value
	err := c.setBin(k, fragmentBuf, true)

	if err != nil {
		return err
	}

	return nil
}

func (c *Cache) getFragmented(retBuf []byte, metadataValue []byte) ([]byte, error) {
	fragmentKey := c.bpool.Get()
	defer c.bpool.Put(fragmentKey)

	// Read and parse metadata value that consist of actual value hash and actual value len
	fragmentKey = metadataValue
	if len(fragmentKey) == 0 {
		return nil, nil
	}
	if len(fragmentKey) != fragmentedEntryKeyLen {
		return nil, nil
	}
	valueHash := common.UnmarshalUint64(fragmentKey)
	valueLen := common.UnmarshalUint64(fragmentKey[8:])

	// Collect the actual value from fragments.
	retBufLen := len(retBuf)
	if n := retBufLen + int(valueLen) - cap(retBuf); n > 0 {
		retBuf = append(retBuf[:cap(retBuf)], make([]byte, n)...)
	}

	retBuf = retBuf[:retBufLen]
	var i uint64
	for uint64(len(retBuf)-retBufLen) < valueLen {
		fragmentKey = common.MarshalUint64(fragmentKey[:0], valueHash)
		fragmentKey = common.MarshalUint64(fragmentKey, i)
		i++
		//ignore "is fragmented" flag because we are collecting fragments
		fragment, _, err := c.getBin(retBuf, fragmentKey)

		if err != nil {
			c.logger.Err("Fragment of the actual value could not found", err)
			return nil, err
		}

		if len(fragment) == len(retBuf) {
			c.logger.Debug("fragment of the actual value could not found")
			return nil, errors.New("fragment of the actual value could not found")
		}
		retBuf = fragment
	}

	// verify the collected fragments.
	v := retBuf[retBufLen:]
	if uint64(len(v)) != valueLen {
		c.logger.Printf("invalid fragmented entry value len — want: %d got: %d", valueLen, len(v))
		return nil, fmt.Errorf("invalid fragmented entry value len — want: %d got: %d", valueLen, len(v))
	}
	h := c.hash.Hash(v)
	if h != valueHash {
		return nil, fmt.Errorf("invalid fragmented value hash want: %d got: %d", valueHash, h)
	}

	return retBuf, nil
}

// maximumShardSizeInBytes computes maximum shard size in bytes
func (c *Cache) maximumShardSizeInBytes() uint64 {
	return uint64((c.maxCacheBytes + c.shardCount - 1) / c.shardCount)
}

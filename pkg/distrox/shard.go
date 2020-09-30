package distrox

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ziyasal/distroxy/internal/pkg/common"

	"github.com/ziyasal/distroxy/internal/pkg/ringo"
)

const (
	entryIndexBytesSize     = 63 // 1 is used store fragmented entry flag
	timestampSizeInBytes    = 8
	entryHeadersSizeInBytes = 12                                    // timestamp + len(k) + len(value)
	defaultKeySizeInBytes   = 16 * 1024                             // 16kb
	defaultValueSizeInBytes = (48 * 1024) - entryHeadersSizeInBytes // (48 * 1024) - 12 KB
	byteSize                = 8
)

var (
	ErrZeroBytesShardSize = errors.New("maxBytes cannot be zero")

	ErrEntrySizeTooBig  = errors.New("key, value with headers size exceeds chunk size")
	ErrEntryKeyTooBig   = errors.New("entry key too big")
	ErrEntryValueTooBig = errors.New("entry value too big")
)

type shard struct {
	rwMutex sync.RWMutex
	ring    *ringo.RingBuf
	// entryIndexes maps hash(k) and fragmented entry flag packed
	//together to position of (ts, k, value) pair in chunks.
	entryIndexes map[uint64]uint64
	// tsBuf used when entry created timestamp is written to headers buffer
	tsBuf []byte

	statsEnabled bool
	ttlInSeconds int64

	logger common.Logger
	clock  common.StoppableClock

	// is a number of successfully found keys
	hits uint64
	// misses is a number of not found keys
	misses uint64
	// delMisses is a number of not deleted keys
	delHits uint64
	// delMisses is a number of not deleted keys
	delMisses uint64
	// collisions is a key collisions counter
	collisions uint64
}

func newShard(
	shardSizeInBytes uint64,
	memBlockSizeInBytes uint64,
	ttlInSeconds int64,
	maxShardSizeInBytes uint64,
	clock common.StoppableClock,
	logger common.Logger,
	statsEnabled bool) (*shard, error) {
	if shardSizeInBytes == 0 {
		return nil, ErrZeroBytesShardSize
	}

	if shardSizeInBytes >= maxShardSizeInBytes {
		return nil, fmt.Errorf(
			"shard size:%d should be smaller than max shard size: %d",
			shardSizeInBytes, maxShardSizeInBytes)
	}

	maxMemBlocks := (shardSizeInBytes + memBlockSizeInBytes - 1) / memBlockSizeInBytes

	s := &shard{}
	s.ring = ringo.NewRingBuf(maxMemBlocks, memBlockSizeInBytes, common.NewDefaultPooled(int(memBlockSizeInBytes)))
	s.entryIndexes = make(map[uint64]uint64)
	s.logger = logger
	s.tsBuf = make([]byte, timestampSizeInBytes)
	s.clock = clock
	s.ttlInSeconds = ttlInSeconds
	s.statsEnabled = statsEnabled

	s.reset()

	return s, nil
}

// "set" stores entry key and value in the ring buffer it also adds entry metadata to map,
// the metadata is hash(key) and fragmented entry [1] flag packed together

// [1] - default chunk size is 64 kb to have low fragmentation
// thus entries bigger than defaults divided into smaller parts
// to fit into the 64kb chunk. The entry then stored with the actual key and the metadata
// about these parts (`fragmented` is 1 in this case) as value.
// When the entry requested, `isFragmentedEntry` flag will be used to determine
// whether processing stored value to collect the parts of actual value is required or not.
func (s *shard) set(k, v []byte, h uint64, fragmented bool) error {
	if len(k) >= defaultKeySizeInBytes {
		return ErrEntryKeyTooBig
	}

	if len(v) >= defaultValueSizeInBytes {
		return ErrEntryValueTooBig
	}

	s.rwMutex.Lock()
	entryHeadersBuf := common.EncodeEntry(k, v, s.clock.Now(), &s.tsBuf)

	entryHeadersLen := uint64(len(entryHeadersBuf) + len(k) + len(v))
	if entryHeadersLen >= s.ring.BlockSize() {
		return ErrEntrySizeTooBig
	}

	currentPosition := s.ring.Write(entryHeadersBuf[:], k, v)

	var isBigEntry uint64 = 0
	if fragmented {
		isBigEntry = 1
	}

	s.entryIndexes[h] = common.PackIntegers(currentPosition, isBigEntry, entryIndexBytesSize)
	s.rwMutex.Unlock()

	return nil
}

//get gets the entry value from shard
// if appendToRetBuf is true then appends the entry value to the retBuf and returns it
func (s *shard) get(retBuf, key []byte, hashOfKey uint64, appendToRetBuf bool) ([]byte, bool, error) {
	s.rwMutex.RLock()
	entryIdx, exists := s.entryIndexes[hashOfKey]
	if !exists {
		s.rwMutex.RUnlock()
		atomic.AddUint64(&s.misses, 1)
		return retBuf, false, ErrEntryNotFound
	}

	// entryIdx consist of the actual index of the entry value and fragmented entry flag
	isFragmentedEntry, entryPosition := common.UnpackIntegers(entryIdx, entryIndexBytesSize)
	entryRingIndex := entryPosition / s.ring.BlockSize()

	if entryRingIndex >= s.ring.Len() {
		s.logger.Printf(
			"corrupted data — chunk index: %d bigger chunks in the ring len: %d",
			entryRingIndex, s.ring.Len())
		s.rwMutex.RUnlock()
		atomic.AddUint64(&s.misses, 1)
		return retBuf, false, ErrEntryNotFound
	}

	entryPosition %= s.ring.BlockSize()

	if entryPosition+entryHeadersSizeInBytes >= s.ring.BlockSize() {
		s.logger.Printf("corrupted data — entry headers:%d from entry index: exceeds chunk size:%d",
			entryHeadersSizeInBytes, entryPosition, s.ring.BlockSize())
		s.rwMutex.RUnlock()
		atomic.AddUint64(&s.misses, 1)
		return retBuf, false, ErrEntryNotFound
	}

	entryHeadersBuf := s.ring.Read(entryRingIndex, entryPosition, entryPosition+entryHeadersSizeInBytes)
	timestamp := int64(common.UnmarshalUint64(entryHeadersBuf[0:timestampSizeInBytes]))

	// Evict on get
	if (s.clock.Now() - timestamp) > s.ttlInSeconds {
		s.rwMutex.RUnlock()

		// acquire lock to delete the item
		s.rwMutex.Lock()
		delete(s.entryIndexes, hashOfKey)
		s.rwMutex.Unlock()

		// increase misses
		if s.statsEnabled {
			atomic.AddUint64(&s.misses, 1)
		}

		return retBuf, false, ErrEntryNotFound
	}

	// get key and value len back
	keyLen := (uint64(entryHeadersBuf[8]) << byteSize) | uint64(entryHeadersBuf[9])
	valLen := (uint64(entryHeadersBuf[10]) << byteSize) | uint64(entryHeadersBuf[11])
	entryPosition += entryHeadersSizeInBytes // (ts,k,v) metadata bytes len

	if entryPosition+keyLen+valLen >= s.ring.BlockSize() {
		s.logger.Printf(
			"corrupted data — entry kv size:%d from the entry index:%d exceeds the chunk size: %d",
			keyLen+valLen, entryPosition, s.ring.BlockSize())
		s.rwMutex.RUnlock()
		return retBuf, false, ErrEntryNotFound
	}

	keyBytes := s.ring.Read(entryRingIndex, entryPosition, entryPosition+keyLen)
	if string(key) == string(keyBytes) {
		entryPosition += keyLen
		if appendToRetBuf {
			valueBytes := s.ring.Read(entryRingIndex, entryPosition, entryPosition+valLen)
			retBuf = append(retBuf, valueBytes...)
		}
		if s.statsEnabled {
			atomic.AddUint64(&s.hits, 1)
		}
	} else if s.statsEnabled {
		atomic.AddUint64(&s.collisions, 1)
	}

	s.rwMutex.RUnlock()
	return retBuf, isFragmentedEntry == 1, nil
}

//del deletes an entry from shard
//(please note that this doesn't delete the entry value,
// it will be overwritten when the ring buffer is full )
func (s *shard) del(h uint64) error {
	if s.statsEnabled {
		atomic.AddUint64(&s.delHits, 1)
	}

	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	if _, ok := s.entryIndexes[h]; !ok {
		if s.statsEnabled {
			atomic.AddUint64(&s.delMisses, 1)
		}
		return ErrEntryNotFound
	}

	delete(s.entryIndexes, h)
	return nil
}

//reset resets shard state and its stats
func (s *shard) reset() {
	s.rwMutex.Lock()

	s.ring.Reset()

	bm := s.entryIndexes
	for k := range bm {
		delete(bm, k)
	}

	atomic.StoreUint64(&s.hits, 0)
	atomic.StoreUint64(&s.misses, 0)
	atomic.StoreUint64(&s.delHits, 0)
	atomic.StoreUint64(&s.delMisses, 0)
	atomic.StoreUint64(&s.collisions, 0)

	s.rwMutex.Unlock()
}

// len returns computes number of entries in shard
func (s *shard) len() uint64 {
	s.rwMutex.RLock()
	length := uint64(len(s.entryIndexes))
	s.rwMutex.RUnlock()

	return length
}

// loadStats adds shard stats to CacheStats
func (s *shard) loadStats(stats *CacheStats) {
	//get
	stats.Hits += atomic.LoadUint64(&s.hits)
	stats.Misses += atomic.LoadUint64(&s.misses)

	stats.Collisions += atomic.LoadUint64(&s.collisions)

	// del
	stats.DelHits += atomic.LoadUint64(&s.delHits)
	stats.DelMisses += atomic.LoadUint64(&s.delMisses)

	s.rwMutex.RLock()
	stats.EntriesCount += uint64(len(s.entryIndexes))
	stats.CacheBytes += s.ring.Cap()
	s.rwMutex.RUnlock()
}

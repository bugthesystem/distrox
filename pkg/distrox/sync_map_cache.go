package distrox

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ziyasal/distroxy/internal/pkg/common"
)

// sync map implementation of the cache to use in benchmarks.
// cache implementations could be refactored to implement
// the same interface but left out of scope.
// I'm happy to discuss about details further.
type syncMapCache struct {
	m     *sync.Map
	count int64

	clock               common.StoppableClock
	ttlInSeconds        int64
	maxKeySizeInBytes   int
	maxValueSizeInBytes int

	// is a number of successfully found keys
	hits uint64
	// misses is a number of not found keys
	misses uint64
	// delMisses is a number of not deleted keys
	delHits uint64
	// delMisses is a number of not deleted keys
	delMisses uint64

	//cache size in bytes
	cacheBytes int64
}

type value struct {
	ts int64
	B  []byte
}

func newSyncMapCache(ttl time.Duration) (*syncMapCache, error) {
	s := &syncMapCache{
		m:            &sync.Map{},
		clock:        common.NewCachedClock(),
		ttlInSeconds: int64(ttl.Seconds()),

		// ignore big entries for now
		maxKeySizeInBytes:   defaultKeySizeInBytes,
		maxValueSizeInBytes: defaultValueSizeInBytes,
	}

	return s, nil
}

func (s *syncMapCache) set(k string, v []byte) error {
	return s.setBin([]byte(k), v)
}
func (s *syncMapCache) get(k string) ([]byte, error) {
	return s.getBin([]byte(k))
}

func (s *syncMapCache) setBin(k []byte, v []byte) error {
	if len(k) >= s.maxKeySizeInBytes {
		return ErrEntryKeyTooBig
	}

	if len(v) >= s.maxValueSizeInBytes {
		return ErrEntryValueTooBig
	}

	val := &value{
		B:  v,
		ts: s.clock.Now(),
	}

	s.m.Store(string(k), val)

	// incr count and bytes size
	atomic.AddInt64(&s.count, 1)
	atomic.AddInt64(&s.cacheBytes, int64(len(v)))
	return nil
}
func (s *syncMapCache) getBin(k []byte) ([]byte, error) {
	if len(k) >= s.maxKeySizeInBytes {
		return nil, ErrEntryKeyTooBig
	}

	v, ok := s.m.Load(string(k))

	if !ok {
		atomic.AddUint64(&s.misses, 1)
		return nil, ErrEntryNotFound
	}

	val := v.(*value)

	if s.clock.Now()-val.ts > s.ttlInSeconds {
		s.m.Delete(string(k))

		atomic.AddInt64(&s.count, -1)
		atomic.AddUint64(&s.misses, 1)

		atomic.AddInt64(&s.cacheBytes, int64(-len(val.B)))
		return nil, ErrEntryNotFound
	}

	atomic.AddUint64(&s.hits, 1)
	return val.B, nil
}

func (s *syncMapCache) del(k string) error {
	return s.delBin([]byte(k))
}

func (s *syncMapCache) delBin(k []byte) error {
	keyStr := string(k)
	v, ok := s.m.Load(keyStr)

	if !ok {
		atomic.AddUint64(&s.delMisses, 1)
		return ErrEntryNotFound
	}

	s.m.Delete(keyStr)
	atomic.AddUint64(&s.delHits, 1)
	atomic.AddInt64(&s.count, -1)
	atomic.AddInt64(&s.cacheBytes, int64(-len(v.(*value).B)))

	return nil
}

// len returns computes number of entries in shard
func (s *syncMapCache) Len() uint64 {
	return uint64(atomic.LoadInt64(&s.count))
}

// loadStats adds shard stats to CacheStats
func (s *syncMapCache) loadStats(stats *CacheStats) {
	//get
	stats.Hits = atomic.LoadUint64(&s.hits)
	stats.Misses = atomic.LoadUint64(&s.misses)

	// del
	stats.DelHits = atomic.LoadUint64(&s.delHits)
	stats.DelMisses = atomic.LoadUint64(&s.delMisses)

	//count and cap
	stats.EntriesCount = uint64(atomic.LoadInt64(&s.count))
	stats.CacheBytes = uint64(atomic.LoadInt64(&s.cacheBytes))
}

func (s *syncMapCache) reset() {
	atomic.StoreUint64(&s.hits, 0)
	atomic.StoreUint64(&s.misses, 0)
	atomic.StoreUint64(&s.delHits, 0)
	atomic.StoreUint64(&s.delMisses, 0)

	atomic.StoreInt64(&s.cacheBytes, 0)
	atomic.StoreInt64(&s.count, 0)
}

func (s *syncMapCache) close() {
	s.clock.Stop()
}

package distrox

// CacheStats stores cache statistics
type CacheStats struct {
	// Hits is a number of successfully found keys
	Hits uint64 `json:"hits"`
	// Misses is a number of not found keys
	Misses uint64 `json:"misses"`

	// DelHits is a number of successfully deleted keys
	DelHits uint64 `json:"delete_hits"`
	// DelMisses is a number of del misses
	DelMisses uint64 `json:"delete_misses"`

	// Collisions is a number of happened key-collisions
	Collisions uint64 `json:"collisions"`

	// Entries is the current number of entries in the cache.
	EntriesCount uint64 `json:"entries_count"`
	// CacheBytes is the current size of the cache in bytes.
	CacheBytes uint64 `json:"cache_bytes"`
}

Distrox
==========
A fast thread-safe in-memory cache library and server that supports a big number of entries in Go

> It can be used as a standalone server or imported as a separate package.

## Example as package
```go
import (
  //... omitted for brevity
	"github.com/ziyasal/distroxy/pkg/distrox"
)

//... omitted for brevity

logger := common.NewZeroLogger(config.app.mode)
cache, err := distrox.NewCache(
	distrox.WithMaxBytes(config.cache.maxBytes),
	distrox.WithShards(config.cache.shards),
	distrox.WithMaxKeySize(config.cache.maxKeySizeInBytes),
	distrox.WithMaxValueSize(config.cache.maxValueSizeInBytes),
	distrox.WithTTL(config.cache.ttlInSeconds),
	distrox.WithLogger(logger),
	distrox.WithStatsEnabled(),
)
  
 // use cache here
```

## Running
```sh
make run
```

## Tests
```
make test
```

## Benchmark
Benchmarks run against 
- raw `sync.Map` (only raw get, set) - 
`Get` performance is baseline here for other implementations

- Cache implementation using `sync.Map`
- Custom sharded cache implementation using ring buffer

There are more optimizations to be made to make read faster for example `3X.XXMB/s`.

**Specs**  
Processor: 2.4 GHz Quad-Core Intel Core i5  
Memory   : 16 GB 2133 MHz LPDDR3  
macOS Catalina 10.15.6
go1.15.2 darwin/amd64

```
❯ make bench
GOMAXPROCS=4 go test ./pkg/distrox/ -bench='Set|Get' -benchtime=10s
goos: darwin
goarch: amd64
pkg: github.com/ziyasal/distroxy/pkg/distrox
BenchmarkSyncMapSet-4              	     303	  37976140 ns/op	   3.45 MB/s	 6846491 B/op	  524728 allocs/op
BenchmarkSyncMapGet-4              	    4905	   2537302 ns/op	  51.66 MB/s	    3286 B/op	     134 allocs/op
BenchmarkSyncMapSetGet-4           	    1078	   9832155 ns/op	  20.34 MB/s	 5208678 B/op	  400123 allocs/op

BenchmarkSyncMapCacheSet-4         	     270	  44550028 ns/op	   2.94 MB/s	 8947434 B/op	  524782 allocs/op
BenchmarkSyncMapCacheGet-4         	    3793	   3180366 ns/op	  20.61 MB/s	    3632 B/op	     104 allocs/op

BenchmarkDistroxCacheSetBin-4      	    1485	   7531564 ns/op	  17.40 MB/s	  257319 B/op	      27 allocs/op
BenchmarkDistroxCacheGetBin-4      	    1909	   5641515 ns/op	  23.23 MB/s	   28658 B/op	      11 allocs/op
BenchmarkDistroxCacheSetGetBin-4   	     787	  15250764 ns/op	  17.19 MB/s	  485540 B/op	      50 allocs/op
PASS
ok  	github.com/ziyasal/distroxy/pkg/distrox	122.740s
```

## Load test
Run `sudo launchctl limit maxfiles 65535 65535` command to increase defaults in case needed.

```sh
pip3 install locust
locust -f scripts/distrox_locust.py --users 1000 --spawn-rate 100

#headless
locust -f scripts/distrox_locust.py  --headless --users 1000 --spawn-rate 100 --run-time 5m
```

## Design Notes
The cache is sharded and has its own locks thus the time spent is reduced
while waiting for locks. Each shard has a map with [1]`hash(key) → packed(position((ts, key, value)), fragmented-flag)`
in the ring buffer, and the ring buffer has 64 KB-size (for having a low-fragmentation) byte slices occupied
by encoded (ts, key, value) entries.

- [1] - uint64 =>  63bits for position and last 1bit for the fragmented flag

There are two cases considered in terms of entry size; 
### Entries fit into default mem-block (64KB)
```sh
|---------------------|-------------------|---------------------|-----------|-------------|
| timestamp bytes — 8 | key len bytes — 2 | value len bytes — 2 | key bytes | value bytes |
|---------------------|-------------------|---------------------|-----------|-------------|
```

### Entries don't fit into default mem-block
For the big entries (k + v + headers > 64 KB), the below approach implemented:
* Split entry into smaller fragments where it can fit into the default memory-block (64KB in our case)
* Calculate the key for each fragment by using fragment index and the value hash and
store the fragment in the cache with the calculated key
* Store the value-hash, and the value-length as a new value (meta-value) with the actual key
(when the entry requested, the stored value (meta-value) will be processed to find out 
the fragments of the actual value)
* Fragmented entry flag for the "meta entry" is set to true (it's "false" for non-fragmented entries). 
Then the flag checked to determine whether processing the entry value required 
or not to collect parts of the actual entry value.

One caveat, storing big entries might require setting a bigger cache sizes to prevent overwriting existing entries.

- Time api (`time.Now`) cached in the clock component and updated every second, this eliminates calls to time api.

### Eviction options
- Cleanup job
- Evict on get

Currently, the `evict on get` approach implemented (also, entries
evicted from the cache on cache size overflow).

Each entry has time created timestamp encoded, the timestamp then
checked whether is life window exceeded or not when access to the
entry happened.  Its deleted from the index map if its lifetime
exceeded, but not from memory.


### Cache Persistence (planned)
Persistence is not implemented yet, but I'm going to discuss how it can be implemented below.

**There are a few persistence options could be considered;**  
- Cache DB persistence performs point-in-time snapshots of the data-set at specified intervals.
- The AOF persistence logs every write operation received by the server, that will be played again at server startup,
reconstructing the original data-set
- Combine both AOF and Cache DB in the same instance, in this case, when the server restarts
the AOF file can be used to reconstruct the original data-set since it is guaranteed to be the most complete.

The following binary format could be used if the first option would be implemented.
**Cache DB Binary Format**  

```sh
----------------------------# CDB is a binary format, without new lines or spaces in the file.
44 49 53 54 52 4f 58        # Magic String "DISTROX"
30 30 30 31                 # 4 digit ASCI CDB Version Number. In this case, version = "0001" = 1
4 bytes                     # Integer DB entry count, high byte first
----------------------------
repeating {
  $ts-bytes
  $key-bytes-length
  $value-bytes-length
  $key-bytes
  $value-bytes
}
----------------------------
8 byte checksum             # CRC 64 checksum of the entire file.
```

## Limitations
- Max cache size should be set when it gets initialized
- Since its uses fixed-size ring buffer on each shard, data will be overwritten when the ring is full
- Each mem-block in the ring buffer is 64 KB mem-size to have a low-fragmentation

## Features out of scope
 - Clustering
 - Versioning (`VectorClock` could be used here)

## Improvements - planned
- Memory blocks could be allocated off-heap
 (`mmap syscall` could be used to access the mapped memory as an array of bytes `[1]`) to prevent taking
 cache size into account by GOGC, and it can be pooled too.

`[1]` - This should be carefully done - if the array is referenced even after
 the memory region is unmapped, this can lead to a segmentation fault
- Export server metrics as Prometheus metrics
- Export cache stats as part of Prometheus metrics (currently its served from `/stats` endpoint)
- Add more tests 
   * cover cache edge cases, 
   * cover more server cases   
   * load test scenarios
- Add deployment configurations (Dockerfile, Helm chart etc)
- Compression support could be added to the server (ie: gzip, brotli)

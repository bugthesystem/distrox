Distrox
==========

## Env Specs
- macOS Catalina 10.15.6
- go1.15.2 darwin/amd64

## Running
```sh
make run
```

## Tests
```
make test
```

## Benchmark
I run benchmarks against 
- raw `sync.Map` (only raw get, set) - 
`Get` performance is baseline here for other implementations

- Cache implementation using `sync.Map`
- Custom sharded cache implementation using ring buffer

There are more optimizations to be made to make read faster for example `3X.XXMB/s`. 
However, I could not spend more time on it but I'd be happy to discuss it further. 

**Specs**  
Processor: 2.4 GHz Quad-Core Intel Core i5  
Memory   : 16 GB 2133 MHz LPDDR3  

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
I run `sudo launchctl limit maxfiles 65535 65535` command to increase defaults.

```sh
pip3 install locust
locust -f scripts/distrox_locust.py --users 1000 --spawn-rate 30
```

## Design Notes
The cache is sharded and has its own locks thus the time spent is reduced
while waiting for locks. Each shard has a map with `hash(key) → position((ts, key, value))`
in the ring buffer, and the ring buffer has 64 KB-size (for having a low-fragmentation) byte slices occupied
by encoded (ts, key, value) entries.

There are two cases considered; 
### Entries fit into default chunk (64KB)
```sh
|---------------------|-------------------|---------------------|-----------|-------------|
| timestamp bytes — 8 | key len bytes — 2 | value len bytes — 2 | key bytes | value bytes |
|---------------------|-------------------|---------------------|-----------|-------------|
```

### Entries don't fit into default mem-block
For the big entries (k + v + headers > 64 KB), the below approach implemented:
* Split entry into smaller parts where it can fit into the default memory-block (64KB in our case)
* Calculate the key for each entry-value-chunk by using chunk number and value hash and
store the entry-value-chunk in the cache with the calculated key
* Store the entry-value-hash, and the entry-value-length as a new value with the actual key
(when the entry requested, the stored value will be processed to find out the parts of the actual entry value)
* Fragmented entry flag for the "meta entry" is set to true (it's "false" for non-fragmented entries). 
Then the flag checked to determine whether processing the entry value required 
or not to collect parts of the actual entry value.

One caveat, storing big entries might require setting a bigger cache sizes to prevent overwriting existing entries.

- Time api is cached in the clock component 
time.Now cached in the clock and updated every second, this eliminates calls to time api.

### Eviction options
- Cleanup job
- Evict on get
Currently, the `evict on get` approach implemented (also, entries
evicted from the cache on cache size overflow).

Each entry has time created timestamp encoded, the timestamp then
checked whether is life window exceeded or not when access to the
entry happened.  Its deleted from the index map if its lifetime
exceeded, but not from memory.


### Cache Persistence (Discussion)
I did not implement persistence, but I'm going to discuss how it can be implemented below.

**There are a few persistence options could be considered;**  
- Cache DB persistence performs point-in-time snapshots of the data-set at specified intervals.
- The AOF persistence logs every write operation received by the server, that will be played again at server startup,
reconstructing the original data-set
- Combine both AOF and Cache DB in the same instance, in this case, when the server restarts
the AOF file can be used to reconstruct the original data-set since it is guaranteed to be the most complete.

I'd go with the first option and use following binary format.
**Cache DB Binary Format**  

```sh
----------------------------# CDB is a binary format, without new lines or spaces in the file.
44 49 53 54 52 4f 58        # Magic String "DISTROX"
30 30 30 31                 # 4 digit ASCII CDB Version Number. In this case, version = "0001" = 1
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
- Since its uses sized-ring buffer on each shard, data will be overwritten when the ring is full
- Each mem-block in the ring buffer is 64 KB mem-size to have a low-fragmentation

## Features out of scope
 - Clustering
 - Versioning (`VectorClock` could be used here)

## Improvements
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
- Use `docker locust` to prevent installing deps
- Integrate exception monitoring tool (ie: Sentry)
- Compression support could be added to server (ie: gzip, brotli)

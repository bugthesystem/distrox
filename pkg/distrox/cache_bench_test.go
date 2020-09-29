package distrox

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ziyasal/distroxy/internal/pkg/common"

	"github.com/stretchr/testify/assert"
)

const itemCount = 131072

// SYNC MAP RAW
func BenchmarkSyncMapSet(b *testing.B) {
	m := sync.Map{}
	b.ReportAllocs()
	b.SetBytes(itemCount)
	b.RunParallel(func(pb *testing.PB) {
		key := []byte("\x00\x00\x00\x00")
		want := "hello world"
		for pb.Next() {
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				m.Store(string(key), want)
			}
		}
	})
}
func BenchmarkSyncMapGet(b *testing.B) {
	c := sync.Map{}
	key := []byte("\x00\x00\x00\x00")
	want := "hello world"
	for i := 0; i < itemCount; i++ {
		key[0]++
		if key[0] == 0 {
			key[1]++
		}
		c.Store(string(key), want)
	}

	b.ReportAllocs()
	b.SetBytes(itemCount)
	b.RunParallel(func(pb *testing.PB) {
		key := []byte("\x00\x00\x00\x00")
		for pb.Next() {
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				got, ok := c.Load(string(key))
				if !ok || got.(string) != string(want) {
					panic(fmt.Errorf("unexpected value — got %q; want %q", got, want))
				}
			}
		}
	})
}
func BenchmarkSyncMapSetGet(b *testing.B) {
	const itemCount = 100_000
	c := sync.Map{}
	b.ReportAllocs()
	b.SetBytes(2 * itemCount)
	b.RunParallel(func(pb *testing.PB) {
		key := []byte("\x00\x00\x00\x00")
		want := "hello world"
		for pb.Next() {
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				c.Store(string(key), want)
			}
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				got, ok := c.Load(string(key))
				if !ok || got.(string) != want {
					panic(fmt.Errorf("unexpected value — got %q; want %q", got, want))
				}
			}
		}
	})
}

// SYNC MAP CACHE
func BenchmarkSyncMapCacheSet(b *testing.B) {
	c, err := newSyncMapCache(10 * time.Minute)
	if err != nil {
		b.Fatalf("could not create cache: %s", err)
	}

	defer c.reset()
	defer c.close()

	b.ReportAllocs()
	b.SetBytes(itemCount)
	b.RunParallel(func(pb *testing.PB) {
		key := []byte("\x00\x00\x00\x00")
		want := []byte("hello world")
		for pb.Next() {
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				err = c.setBin(key, want)
				if err != nil {
					b.Fatalf("could not set: %s", err)
				}
			}
		}
	})
}
func BenchmarkSyncMapCacheGet(b *testing.B) {
	const items = 65536
	c, err := newSyncMapCache(10 * time.Minute)

	if err != nil {
		b.Fatalf("could not create cache: %s", err)
	}

	defer c.reset()
	defer c.close()

	key := []byte("\x00\x00\x00\x00")
	want := []byte("hello world")
	for i := 0; i < items; i++ {
		key[0]++
		if key[0] == 0 {
			key[1]++
		}
		err = c.setBin(key, want)
		if err != nil {
			b.Fatalf("could not set: %s", err)
		}
	}

	b.ReportAllocs()
	b.SetBytes(items)
	b.RunParallel(func(pb *testing.PB) {
		var got []byte
		k := []byte("\x00\x00\x00\x00")
		for pb.Next() {
			for i := 0; i < items; i++ {
				k[0]++
				if k[0] == 0 {
					k[1]++
				}
				got, err = c.getBin(k)
				assert.Nil(b, err)
				if string(got) != string(want) {
					panic(fmt.Errorf("invalid value — got %q; want %q", got, want))
				}
			}
		}
	})
}

// DISTROX CACHE
func BenchmarkDistroxCacheSetBin(b *testing.B) {
	c, err := NewCache(
		WithMaxBytes(64*1024*1024),
		WithClock(common.NewCachedClock()),
	)
	if err != nil {
		b.Fatalf("could not create cache: %s", err)
	}

	defer c.Reset()
	defer c.Close()

	b.ReportAllocs()
	b.SetBytes(itemCount)
	b.RunParallel(func(pb *testing.PB) {
		key := []byte("\x00\x00\x00\x00")
		want := []byte("hello world")
		for pb.Next() {
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				err = c.SetBin(key, want)
				if err != nil {
					b.Fatalf("could not set: %s", err)
				}
			}
		}
	})
}
func BenchmarkDistroxCacheGetBin(b *testing.B) {
	c, err := NewCache(
		WithMaxBytes(1024*1024*1024),
		WithClock(common.NewCachedClock()),
	)

	if err != nil {
		b.Fatalf("could not create cache: %s", err)
	}

	defer c.Reset()
	defer c.Close()

	key := []byte("\x00\x00\x00\x00")
	want := []byte("hello world")
	for i := 0; i < itemCount; i++ {
		key[0]++
		if key[0] == 0 {
			key[1]++
		}
		err = c.SetBin(key, want)
		if err != nil {
			b.Fatalf("could not set: %s", err)
		}
	}

	b.ReportAllocs()
	b.SetBytes(itemCount)
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		key := []byte("\x00\x00\x00\x00")
		for pb.Next() {
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				buf, err = c.GetBin(buf[:0], key)
				assert.Nil(b, err)
				if string(buf) != string(want) {
					panic(fmt.Errorf("invalid value — got %q; want %q", buf, want))
				}
			}
		}
	})
}
func BenchmarkDistroxCacheSetGetBin(b *testing.B) {
	c, err := NewCache(
		WithMaxBytes(64*1024*1024),
		WithClock(common.NewCachedClock()),
	)

	if err != nil {
		b.Fatalf("cannot create cache: %s", err)
	}

	defer c.Reset()
	defer c.Close()

	b.ReportAllocs()
	b.SetBytes(2 * itemCount)
	b.RunParallel(func(pb *testing.PB) {
		key := []byte("\x00\x00\x00\x00")
		want := []byte("hello world")
		var got []byte
		for pb.Next() {
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				err = c.SetBin(key, want)
				if err != nil {
					b.Fatalf("could not set: %s", err)
				}
			}
			for i := 0; i < itemCount; i++ {
				key[0]++
				if key[0] == 0 {
					key[1]++
				}
				got, err = c.GetBin(got[:0], key)
				assert.Nil(b, err)
				if string(got) != string(want) {
					panic(fmt.Errorf("invalid value — got %q; want %q", got, want))
				}
			}
		}
	})
}

package common

import (
	"sync"
)

//Pooled simple bytes pool interface
type Pooled interface {
	Get() []byte
	Put(b []byte)
}

// syncPooled is default sync.Pool based implementation
type syncPooled struct {
	blockSize int
	pool      *sync.Pool
}

func NewDefaultPooled(blockSize int) Pooled {
	p := syncPooled{
		blockSize: blockSize,
	}

	p.pool = new(sync.Pool)

	return &p
}

// get gets a chunk from the pool or creates a new one if reuse failed.
func (p *syncPooled) Get() []byte {
	v := p.pool.Get()
	if v != nil {
		return v.([]byte)
	}

	return make([]byte, p.blockSize)
}

// put puts a chunk to reuse pool if it can be reused.
func (p *syncPooled) Put(b []byte) {
	size := cap(b)
	if p.blockSize != 0 && size < p.blockSize {
		return
	}

	p.pool.Put(b[:0])
}

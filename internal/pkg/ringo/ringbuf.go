package ringo

import "github.com/ziyasal/distroxy/internal/pkg/common"

// RingBuf is a sized-ring buffer consists of sized blocks.
type RingBuf struct {
	blocks    [][]byte
	blockSize uint64

	// writeCursor points to blocks for writing the next slice
	writeCursor uint64
	pool        common.Pooled
}

func (r *RingBuf) Reset() {
	blocks := r.blocks

	for i := range blocks {

		r.pool.Put(blocks[i])
		blocks[i] = nil
	}

	r.writeCursor = 0
}

func (r *RingBuf) Pos() uint64 {
	return r.writeCursor
}

func (r *RingBuf) Len() uint64 {
	return uint64(len(r.blocks))
}

func (r *RingBuf) BlockSize() uint64 {
	return r.blockSize
}

func (r *RingBuf) Read(index uint64, lowBound uint64, highBound uint64) []byte {
	return r.blocks[index][lowBound:highBound]
}

func (r *RingBuf) Write(blobs ...[]byte) uint64 {
	var blobLen uint64
	for _, b := range blobs {
		blobLen += uint64(len(b))
	}

	currentPosition := r.Pos()
	nextPosition := currentPosition + blobLen
	blockIdx := currentPosition / r.blockSize
	newBlockIndex := nextPosition / r.blockSize

	if newBlockIndex > blockIdx {
		if newBlockIndex >= r.Len() {
			currentPosition = 0
			nextPosition = blobLen
			blockIdx = 0
		} else {
			currentPosition = newBlockIndex * r.blockSize
			nextPosition = currentPosition + blobLen
			blockIdx = newBlockIndex
		}

		// reset block
		r.blocks[blockIdx] = r.blocks[blockIdx][:0]
	}

	block := r.blocks[blockIdx]
	if block == nil {
		block := r.pool.Get()
		block = block[:0]
	}
	for _, b := range blobs {
		block = append(block, b...)
	}

	r.blocks[blockIdx] = block

	r.writeCursor = nextPosition

	return currentPosition
}

func (r *RingBuf) Cap() uint64 {
	var c uint64
	for _, block := range r.blocks {
		c += uint64(cap(block))
	}

	return c
}

func NewRingBuf(blocks uint64, blockSize uint64, pool common.Pooled) *RingBuf {
	return &RingBuf{
		blocks:    make([][]byte, blocks),
		blockSize: blockSize,
		pool:      pool,
	}
}

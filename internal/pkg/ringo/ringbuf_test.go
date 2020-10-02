package ringo

import (
	"testing"

	"github.com/ziyasal/distroxy/internal/pkg/common"

	"github.com/stretchr/testify/assert"
)

func TestRingBuf_ReadWrite(t *testing.T) {
	r := NewRingBuf(1024, 1024, common.NewDefaultPooled(1024))

	// put big value so that next entry can get to another mem-block
	want := make([]byte, 1010)

	pos := r.Write(want)
	got := r.Read(0, pos, pos+uint64(len(want)))
	assert.Equal(t, want, got)

	want = []byte("hello lovely world!")
	pos = r.Write(want)

	indexToRead := pos / r.BlockSize()
	pos %= r.BlockSize()
	got = r.Read(indexToRead, pos, pos+uint64(len(want)))

	assert.Equal(t, want, got)
}

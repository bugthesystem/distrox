package ringo

import (
	"testing"

	"github.com/ziyasal/distroxy/internal/pkg/common"

	"github.com/stretchr/testify/assert"
)

func TestRingBuf_ReadWrite(t *testing.T) {
	r := NewRingBuf(1024, 1024, common.NewDefaultPooled(1024))

	// put big value so that next entry can get to another mem-block
	expected := make([]byte, 1010)

	pos := r.Write(expected)
	actual := r.Read(0, pos, pos+uint64(len(expected)))
	assert.Equal(t, expected, actual)

	expected = []byte("hello lovely world!")
	pos = r.Write(expected)

	indexToRead := pos / r.BlockSize()
	pos %= r.BlockSize()
	actual = r.Read(indexToRead, pos, pos+uint64(len(expected)))

	assert.Equal(t, expected, actual)
}

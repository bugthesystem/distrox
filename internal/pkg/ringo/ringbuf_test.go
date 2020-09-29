package ringo

import (
	"testing"

	"github.com/ziyasal/distroxy/internal/pkg/common"

	"github.com/stretchr/testify/assert"
)

func TestRingBuf_ReadWrite(t *testing.T) {
	r := NewRingBuf(1024, 1024, common.NewDefaultPooled(1024))

	expected := []byte("bar")

	pos := r.Write(expected)
	actual := r.Read(0, pos, pos+uint64(len(expected)))
	assert.Equal(t, expected, actual)

	expected = []byte("baz")
	pos = r.Write(expected)
	actual = r.Read(0, pos, pos+uint64(len(expected)))

	assert.Equal(t, expected, actual)
}

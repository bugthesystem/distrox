package common

import "github.com/zeebo/xxh3"

type Hasher interface {
	Hash(bytes []byte) uint64
	HashStr(str string) uint64
}

type defaultHasher struct {
}

func (h *defaultHasher) HashStr(str string) uint64 {
	return xxh3.HashString(str)
}

func (h *defaultHasher) Hash(bytes []byte) uint64 {
	return xxh3.Hash(bytes)
}

func NewDefaultHasher() Hasher {
	return &defaultHasher{}
}

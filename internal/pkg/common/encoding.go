package common

import (
	"encoding/binary"
)

func EncodeEntry(key []byte, value []byte, timestamp int64, tsBuff *[]byte) [12]byte {
	var kvLenBuf [12]byte

	blob := *tsBuff

	binary.LittleEndian.PutUint64(blob, uint64(timestamp))

	copy(kvLenBuf[0:], blob)

	kvLenBuf[8] = byte(uint16(len(key)) >> 8)
	kvLenBuf[9] = byte(len(key))

	kvLenBuf[10] = byte(uint16(len(value)) >> 8)
	kvLenBuf[11] = byte(len(value))

	return kvLenBuf
}

// PackIntegers packs two integers to one by using size bits.
// please note that same size bits must be used to unpack
func PackIntegers(x uint64, y uint64, sizeBits int) uint64 {
	return x | (y << sizeBits)
}

// UnpackIntegers unpacks two integers from the packed integer
// please note that the same "size bits" that is used while packing must be used
func UnpackIntegers(packed uint64, sizeBits int) (uint64, uint64) {
	x := packed >> sizeBits
	y := packed & ((1 << sizeBits) - 1)

	return x, y
}

// MarshalUint64 encode uint64 to byte array
func MarshalUint64(dst []byte, u uint64) []byte {
	return append(dst,
		byte(u>>56),
		byte(u>>48),
		byte(u>>40),
		byte(u>>32),
		byte(u>>24),
		byte(u>>16),
		byte(u>>8),
		byte(u))
}

// UnmarshalUint64 decode uint64 from b byte array
func UnmarshalUint64(src []byte) uint64 {
	//validate size
	_ = src[7]

	return uint64(src[0])<<56 |
		uint64(src[1])<<48 |
		uint64(src[2])<<40 |
		uint64(src[3])<<32 |
		uint64(src[4])<<24 |
		uint64(src[5])<<16 |
		uint64(src[6])<<8 |
		uint64(src[7])
}

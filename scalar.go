package yarp

import (
	"io"
)

// Encodes at most 64 bits of a given number into a buffer, returning the
// encoded buffer. Callers must manually add required headers into the first 4
// bits of the first byte in the provided buffer.
func encodeInteger(value uint64) []byte {
	const maxLen = 16
	pos := maxLen - 1
	data := make([]byte, maxLen)
	for (value << 1) > 0x7 {
		data[pos] = uint8(value&0x7F) << 1
		if pos != maxLen-1 {
			data[pos] = data[pos] | 0x1
		}
		pos--
		value >>= 7
	}
	data[pos] = uint8(value<<1) & 0x7
	if pos < maxLen-1 {
		data[pos] |= 0x1
	}
	return data[pos:]
}

func encodeInt(value int64) []byte {
	data := encodeInteger(uint64(value))
	data[0] = data[0] | 0x30
	return data
}

func encodeUint(value uint64) []byte {
	data := encodeInteger(value)
	data[0] = data[0] | 0x20
	return data
}

func encodeBool(value bool) []byte {
	if value {
		return []byte{0x30}
	} else {
		return []byte{0x20}
	}
}

func decodeScalar(header byte, reader io.Reader) (signed bool, value uint64, err error) {
	value = uint64(header&0xE) >> 1
	signed = header&0x10 == 0x10
	if header&0x1 != 0x1 {
		return
	}
	b := []byte{0x00}
	for {
		value <<= 7
		if _, err = reader.Read(b); err != nil {
			return
		}
		value |= uint64(b[0]) >> 1
		if b[0]&0x01 != 0x01 {
			break
		}
	}
	return
}

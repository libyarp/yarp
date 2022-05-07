package yarp

import (
	"encoding/binary"
	"io"
	"math"
)

func encodeFloat32(value float32) []byte {
	header := uint8(0x40)
	if value == 0 {
		header |= 0x8
		return []byte{header}
	}
	data := make([]byte, 5)
	data[0] = header
	v := math.Float32bits(value)
	binary.LittleEndian.PutUint32(data[1:], v)
	return data
}

func encodeFloat64(value float64) []byte {
	header := uint8(0x50)
	if value == 0 {
		header |= 0x8
		return []byte{header}
	}
	data := make([]byte, 9)
	data[0] = header
	v := math.Float64bits(value)
	binary.LittleEndian.PutUint64(data[1:], v)
	return data
}

func decodeFloat(header byte, reader io.Reader) (bits int, value float64, err error) {
	bits = 32
	if header&0x10 == 0x10 {
		bits = 64
	}
	isZero := header&0x8 == 0x8
	if isZero {
		return
	}
	buffer := make([]byte, bits/8)
	if _, err = io.ReadFull(reader, buffer); err != nil {
		return
	}
	if bits == 32 {
		v := binary.LittleEndian.Uint32(buffer)
		value = float64(math.Float32frombits(v))
	} else {
		v := binary.LittleEndian.Uint64(buffer)
		value = math.Float64frombits(v)
	}
	return
}

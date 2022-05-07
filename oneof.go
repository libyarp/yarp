package yarp

import (
	"fmt"
	"io"
	"reflect"
)

// OneOfValue represents an oneof value that has not been applied to a struct
// field. The value includes both the Index of the oneof field that should be
// set, and a Data value including the value that should be applied to such
// field.
type OneOfValue struct {
	Index int
	Data  interface{}
}

func encodeOneOf(ov *OneOfValue) ([]byte, error) {
	if t := reflect.TypeOf(ov.Data); !canEncode(t) {
		return nil, fmt.Errorf("cannot encode value of type %s", t)
	}

	rv := reflect.ValueOf(ov.Data)
	v, err := encode(rv)
	if err != nil {
		return nil, err
	}
	idx := encodeUint(uint64(ov.Index))
	v = append(idx, v...)
	head := encodeInteger(uint64(len(v)))
	head[0] |= 0xE0
	return append(head, v...), nil
}

func decodeOneOf(header byte, r io.Reader) (*OneOfValue, error) {
	_, size, err := decodeScalar(header, r)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return nil, nil
	} else if size >= sizeLimit {
		return nil, ErrSizeTooLarge
	}
	reader := io.LimitReader(r, int64(size))
	h := []byte{0x0}
	if _, err = io.ReadFull(reader, h); err != nil {
		return nil, err
	}
	_, idx, err := decodeScalar(h[0], reader)
	if err != nil {
		return nil, err
	}
	_, val, err := Decode(reader)
	if err != nil {
		return nil, err
	}

	return &OneOfValue{Index: int(idx), Data: val}, nil
}

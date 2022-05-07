package yarp

import (
	"fmt"
	"io"
)

// Decode takes an io.Reader and attempts to decode it as either a primitive
// type, or a registered message. Decode returns an error in case the provided
// stream contains an unregistered message.
// Decode does not close r.
func Decode(r io.Reader) (t Type, ret interface{}, err error) {
	defer func() {
		if rawErr := recover(); rawErr != nil {
			if innerErr, ok := rawErr.(error); ok {
				err = innerErr
				return
			}

			err = fmt.Errorf("unexpected error during decode operation: %s", rawErr)
		}
	}()

	header := []byte{0x00}
	if _, err := r.Read(header); err != nil {
		return Invalid, nil, err
	}
	switch detectType(header[0]) {
	case Void:
		return Void, nil, nil
	case Scalar:
		s, v, err := decodeScalar(header[0], r)
		if err != nil {
			return Scalar, nil, err
		}
		if s {
			return Scalar, int64(v), nil
		}
		return Scalar, v, nil
	case Float:
		bits, v, err := decodeFloat(header[0], r)
		if err != nil {
			return Float, nil, err
		}
		if bits == 32 {
			return Float, float32(v), nil
		}
		return Float, v, nil
	case Array:
		arr, err := decodeArray(header[0], r)
		return Array, arr, err
	case String:
		str, err := decodeString(header[0], r)
		return String, str, err
	case Struct:
		str, err := decodeStructToConcrete(header[0], r)
		return Struct, str, err
	case Map:
		m, err := decodeMap(header[0], r)
		return Map, m, err
	case OneOf:
		oo, err := decodeOneOf(header[0], r)
		return OneOf, oo, err
	default:
		return Invalid, nil, ErrInvalidType
	}
}

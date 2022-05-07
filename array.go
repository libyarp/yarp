package yarp

import (
	"fmt"
	"io"
	"reflect"
)

func encodeArray(val reflect.Value) ([]byte, error) {
	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("encodeArray invoked for non-array type %s", val.String())
	}
	if val.Len() == 0 {
		return []byte{0x60}, nil
	}

	sliceLen := val.Len()
	sliceType := val.Index(0).Type()
	// Type-check
	for i := 0; i < sliceLen; i++ {
		if val.Index(i).Type() != sliceType {
			return nil, ErrNonHomogeneousArray
		}
	}

	var buf []byte
	for i := 0; i < sliceLen; i++ {
		b, err := encode(val.Index(i))
		if err != nil {
			return nil, err
		}
		buf = append(buf, b...)
	}
	header := encodeInteger(uint64(len(buf)))
	header[0] = header[0] | 0x60
	return append(header, buf...), nil
}

func decodeArray(header byte, r io.Reader) ([]interface{}, error) {
	var data []interface{}
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
	for {
		t, v, err := Decode(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if t == Struct {
			v = reflect.ValueOf(v).Elem().Interface()
		}
		data = append(data, v)
	}

	return data, nil
}

package yarp

import (
	"fmt"
	"io"
	"reflect"
)

// MapValue represents a map that has not been transformed into a map[T]U.
// For each Keys[n], the associated value can be obtained through Values[n].
type MapValue struct {
	Keys   []interface{}
	Values []interface{}
}

var reflectedMapValue = reflect.TypeOf(&MapValue{})

func encodeMap(val reflect.Value) ([]byte, error) {
	if val.Kind() != reflect.Map {
		return nil, fmt.Errorf("encodeMap invoked for non-map type %s", val.String())
	}

	kType := val.Type().Key()
	vType := val.Type().Elem()

	if !validMapKeyType(kType.Kind()) {
		return nil, fmt.Errorf("encodeMap invoked for map with non-encodable key type %s", kType)
	}

	if !canEncode(vType) {
		return nil, fmt.Errorf("cannot encode map value type %s", vType)
	}

	var keys []byte
	var values []byte

	iter := val.MapRange()
	for iter.Next() {
		k, err := encode(iter.Key())
		if err != nil {
			return nil, err
		}
		v, err := encode(iter.Value())
		if err != nil {
			return nil, err
		}
		keys = append(keys, k...)
		values = append(values, v...)
	}

	kLen := len(keys)
	vLen := len(values)
	mLen := val.Len()

	if mLen == 0 {
		head := encodeInteger(0)
		head[0] |= 0xc0
		return head, nil
	}

	kLenBytes := encodeUint(uint64(kLen))
	vLenBytes := encodeUint(uint64(vLen))

	keys = append(kLenBytes, keys...)
	values = append(vLenBytes, values...)
	mLen = len(keys) + len(values)
	head := encodeInteger(uint64(mLen))
	head[0] |= 0xc0
	head = append(head, keys...)
	return append(head, values...), nil
}

func decodeMap(header byte, r io.Reader) (*MapValue, error) {
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
	b := []byte{0x00}
	if _, err = reader.Read(b); err != nil {
		return nil, err
	}

	_, keyLen, err := decodeScalar(b[0], r)
	if err != nil {
		return nil, err
	}
	keyReader := io.LimitReader(r, int64(keyLen))
	mapVal := &MapValue{}
	keyType := Invalid
	for {
		t, v, err := Decode(keyReader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if keyType == Invalid {
			keyType = t
		} else if keyType != t {
			return nil, fmt.Errorf("non-homogeneous map key type")
		}
		mapVal.Keys = append(mapVal.Keys, v)
	}

	if _, err = reader.Read(b); err != nil {
		return nil, err
	}
	_, valLen, err := decodeScalar(b[0], r)
	if err != nil {
		return nil, err
	}
	valReader := io.LimitReader(r, int64(valLen))
	valType := Invalid
	for {
		t, v, err := Decode(valReader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if valType == Invalid {
			valType = t
		} else if valType != t {
			return nil, fmt.Errorf("non-homogeneous map value type")
		}
		mapVal.Values = append(mapVal.Values, v)
	}

	if len(mapVal.Keys) != len(mapVal.Values) {
		return nil, fmt.Errorf("uneven map values")
	}

	return mapVal, nil
}

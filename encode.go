package yarp

import (
	"fmt"
	"reflect"
)

func encode(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Slice:
		return encodeArray(v)
	case reflect.String:
		return encodeString(v.String()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return encodeUint(v.Uint()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encodeInt(v.Int()), nil
	case reflect.Bool:
		return encodeBool(v.Bool()), nil
	case reflect.Float32:
		return encodeFloat32(float32(v.Float())), nil
	case reflect.Float64:
		return encodeFloat64(v.Float()), nil
	case reflect.Pointer:
		if v.IsNil() {
			return encodeVoid(), nil
		}
		return encode(v.Elem())
	case reflect.Struct:
		return encodeStruct(v)
	case reflect.Map:
		return encodeMap(v)
	default:
		return nil, fmt.Errorf("cannot encode type %s", v.Kind())
	}
}

// Encode takes an arbitrary value and encodes it into a byte slice.
func Encode(v interface{}) (ret []byte, err error) {
	defer func() {
		if rawErr := recover(); rawErr != nil {
			if innerErr, ok := rawErr.(error); ok {
				err = innerErr
				return
			}

			err = fmt.Errorf("unexpected error during decode operation: %s", rawErr)
		}
	}()
	return encode(reflect.ValueOf(v))
}

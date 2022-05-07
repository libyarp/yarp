package yarp

import "reflect"

var registry = map[uint64]reflect.Type{}

// TryRegisterStructType takes an arbitrary number of StructValuer instances,
// validates them, and registers them to be able to decode streams into their
// respective types. Returns an error in case a struct is invalid.
func TryRegisterStructType(v ...StructValuer) error {
	for _, v := range v {
		reflected := reflect.TypeOf(v)
		if reflected.Kind() == reflect.Pointer {
			reflected = reflected.Elem()
		}
		_, err := validateAndExtractStruct(reflected)
		if err != nil {
			return err
		}
		registry[v.YarpID()] = reflected
	}
	return nil
}

// RegisterStructType works like TryRegisterStructType, but panics instead of
// returning an error.
func RegisterStructType(v ...StructValuer) {
	if err := TryRegisterStructType(v...); err != nil {
		panic(err)
	}
}

func resetRegistry() {
	for k := range registry {
		delete(registry, k)
	}
}

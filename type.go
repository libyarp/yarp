package yarp

import "reflect"

//go:generate stringer -type=Type

const sizeLimit = uint64(2e+9)

// Type represents the types present in a YARP stream.
type Type int

const (
	// Invalid represents an unknown or corrupt type.
	Invalid Type = iota

	// Void represents a void (empty) type.
	Void

	// Scalar represents all signed and unsigned integer values, along with
	// booleans.
	Scalar

	// Float represents both 32 and 64-bit float values.
	Float

	// Array represents a list of a single type.
	Array

	// Struct represents a user-defined structure.
	Struct

	// String represents a UTF-8 character array
	String

	// Map represents an associative array between two types.
	Map

	// OneOf represents a field containing one of several possible types.
	OneOf
)

func detectType(b byte) Type {
	v, ok := map[byte]Type{
		0x0: Void,
		0x1: Scalar,
		0x2: Float,
		0x3: Array,
		0x4: Struct,
		0x5: String,
		0x6: Map,
		0x7: OneOf,
	}[b>>5]
	if !ok {
		return Invalid
	}
	return v
}

var validMapKeyTypeLookup = map[reflect.Kind]bool{
	reflect.String: true,
	reflect.Uint:   true,
	reflect.Uint8:  true,
	reflect.Uint16: true,
	reflect.Uint32: true,
	reflect.Uint64: true,
	reflect.Int:    true,
	reflect.Int8:   true,
	reflect.Int16:  true,
	reflect.Int32:  true,
	reflect.Int64:  true,
}

func validMapKeyType(k reflect.Kind) bool {
	_, ok := validMapKeyTypeLookup[k]
	return ok
}

func canEncode(t reflect.Type) bool {
	// validMapKeyType covers pretty much all scalar types (except bool), and
	// string. So in case t's Kind is covered by it, we're good to encode it.
	if validMapKeyType(t.Kind()) {
		return true
	}

	switch t.Kind() {
	case reflect.Float32, reflect.Float64, reflect.Bool:
		return true
	case reflect.Slice, reflect.Pointer:
		return canEncode(t.Elem())
	case reflect.Map:
		return canEncodeMap(t)
	case reflect.Struct:
		return canEncodeStruct(t)
	}

	return false
}

func canEncodeMap(t reflect.Type) bool {
	return validMapKeyType(t.Key().Kind()) && canEncode(t.Elem())
}

func canEncodeStruct(t reflect.Type) bool {
	if !t.Implements(reflectedValuer) {
		return false
	}
	sf, ok := t.FieldByName("Structure")
	if !ok {
		return false
	}
	return sf.Type == reflectedStructure
}

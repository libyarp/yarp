package yarp

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// StructValuer represents a struct that can be encoded into a YARP stream.
type StructValuer interface {
	YarpID() uint64
	YarpPackage() string
	YarpStructName() string
}

// UnknownField represents a field present in a stream, but not handled by the
// known structure type.
type UnknownField struct {
	Index int
	Type  Type
	Data  interface{}
}

// Structure contains a list of UnknownFields obtained during the decoding
// process.
type Structure struct {
	UnknownFields []UnknownField
}

var reflectedValuer = reflect.TypeOf((*StructValuer)(nil)).Elem()
var reflectedStructure = reflect.TypeOf(&Structure{})

type encodedStruct struct {
	id     uint64
	values []interface{}
	types  []Type
}

type structField struct {
	Index        int
	OneOf        bool
	Field        reflect.StructField
	OneOfIndexes map[int]reflect.StructField
}

func validateAndExtractStruct(t reflect.Type) ([]structField, error) {
	if !t.Implements(reflectedValuer) {
		return nil, ErrIncompatibleStruct
	}

	_, ok := t.FieldByName("Structure")
	if !ok {
		return nil, ErrIncompleteStruct
	}

	fields := map[int]structField{}
	minField := 10000
	maxField := -1
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, ok := f.Tag.Lookup("index")
		if !ok {
			continue
		}
		oneOfIndex := ""
		if strings.ContainsRune(tag, ',') {
			components := strings.Split(tag, ",")
			tag, oneOfIndex = components[0], components[1]
			// For a OneOf field, we require it to be declared as a pointer
			if f.Type.Kind() != reflect.Pointer {
				return nil, fmt.Errorf("expected OneOf fields to use a pointer")
			}
		}
		i, err := strconv.Atoi(tag)
		if err != nil {
			return nil, ErrInvalidTag
		}
		ef, ok := fields[i]
		if ok && (oneOfIndex == "" || !ef.OneOf) {
			return nil, ErrDuplicatedFieldIndex
		}
		if i < minField {
			minField = i
		}
		if i > maxField {
			maxField = i
		}
		if ok {
			ooIndex, err := strconv.Atoi(oneOfIndex)
			if err != nil {
				return nil, ErrInvalidTag
			}
			ef.OneOfIndexes[ooIndex] = f
		} else {
			sf := structField{
				Index:        i,
				OneOf:        oneOfIndex != "",
				Field:        f,
				OneOfIndexes: nil,
			}
			if sf.OneOf {
				ooIndex, err := strconv.Atoi(oneOfIndex)
				if err != nil {
					return nil, ErrInvalidTag
				}

				sf.OneOfIndexes = map[int]reflect.StructField{
					ooIndex: f,
				}
			}
			fields[i] = sf
		}
	}

	// We can continue as long as our index begins at zero, and have no gaps
	// between items.
	if minField != 0 {
		return nil, ErrMinFieldNotZero
	}

	for i := 0; i <= maxField; i++ {
		_, ok = fields[i]
		if !ok {
			return nil, ErrFieldGap
		}
	}

	allFields := make([]structField, maxField+1)
	for i := 0; i <= maxField; i++ {
		allFields[i] = fields[i]
	}
	return allFields, nil
}

func encodeStruct(v reflect.Value) ([]byte, error) {
	fields, err := validateAndExtractStruct(v.Type())
	if err != nil {
		return nil, err
	}
	// Encode all values in order
	var body []byte
	for _, f := range fields {
		var b []byte
		if f.OneOf {
			oo := &OneOfValue{Index: -1}
			for k, f := range f.OneOfIndexes {
				val := v.FieldByIndex(f.Index)
				if val.IsNil() {
					continue
				}
				oo.Index = k
				oo.Data = val.Interface()
				break
			}
			b, err = encodeOneOf(oo)
		} else {
			b, err = encode(v.FieldByIndex(f.Field.Index))
		}
		if err != nil {
			return nil, err
		}
		body = append(body, b...)
	}
	header := encodeInteger(uint64(len(body)) + 8) // ID + body
	header[0] |= 0x80
	id := make([]byte, 8)
	binary.LittleEndian.PutUint64(id, v.Interface().(StructValuer).YarpID())
	header = append(header, id...)
	return append(header, body...), nil
}

func decodeStruct(header byte, r io.Reader) (*encodedStruct, error) {
	_, size, err := decodeScalar(header, r)
	if err != nil {
		return nil, err
	}

	if size >= sizeLimit {
		return nil, ErrSizeTooLarge
	}
	r = io.LimitReader(r, int64(size))
	id := make([]byte, 8)
	if _, err := io.ReadFull(r, id); err != nil {
		return nil, err
	}
	str := &encodedStruct{
		id: binary.LittleEndian.Uint64(id),
	}
	for {
		t, v, err := Decode(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		str.values = append(str.values, v)
		str.types = append(str.types, t)
	}

	return str, nil
}

func decodeStructToConcrete(b byte, r io.Reader) (interface{}, error) {
	str, err := decodeStruct(b, r)
	if err != nil {
		return nil, err
	}
	t, ok := registry[str.id]
	if !ok {
		return str, ErrUnknownStructType
	}
	inst := reflect.New(t)

	setInst := inst.Elem()
	allFields, err := validateAndExtractStruct(t)
	if err != nil {
		return nil, err
	}

	vLen := len(str.values)
	var unknownFields []UnknownField
	maxI := 0
	for i, f := range allFields {
		if i > vLen {
			break
		}
		v := str.values[i]

		maxI = i
		if f.OneOf {
			oo, ok := v.(*OneOfValue)
			if ok {
				if oo.Index == -1 {
					// No one is set. Just continue.
					continue
				}
				field, ok := f.OneOfIndexes[oo.Index]
				val := oo.Data
				// Here's a catch: All OneOf values are pointers, but oo.Data
				// will never contain a pointer. For that, we create a new
				// pointer, set its value, and pass it to setValue.
				ptr := reflect.New(field.Type.Elem())
				ptr.Elem().Set(reflect.ValueOf(val))
				if ok && setValue(setInst, field, ptr) {
					if hasF, ok := t.FieldByName("Has" + field.Name); ok && hasF.Type.Kind() == reflect.Bool {
						setInst.FieldByIndex(hasF.Index).SetBool(true)
					}
					continue
				}
			}
		} else if setValue(setInst, f.Field, v) {
			continue
		}

		unknownFields = append(unknownFields, UnknownField{
			Index: i,
			Type:  str.types[i],
			Data:  v,
		})
	}

	for i, v := range str.values[maxI+1:] {
		t := str.types[i]
		unknownFields = append(unknownFields, UnknownField{
			Index: maxI + i,
			Type:  t,
			Data:  v,
		})
	}

	sf, _ := t.FieldByName("Structure")
	setInst.FieldByIndex(sf.Index).Set(reflect.ValueOf(&Structure{
		UnknownFields: unknownFields,
	}))
	return inst.Interface(), nil
}

func setValue(into reflect.Value, fd reflect.StructField, value interface{}) bool {
	var rv reflect.Value
	if v, ok := value.(reflect.Value); ok {
		rv = v
	} else {
		rv = reflect.ValueOf(value)
	}

	switch {
	case fd.Type.Kind() == reflect.Pointer && !rv.IsValid():
		// nil for a pointer, there's not much to do here. This case is only
		// here to prevent the switch from going into the default case.
		// Feel free to rest at this bonfire, traveller.
		//                          xg,
		//                         1 I
		//                         #F`
		//                        ,#6
		//                        k#
		//                       ,k!
		//                       [E
		//                      ,N&
		//                  *""NR#==~`
		//                     [M}
		//                     W0
		//                   ,Q#!    **
		//               *   44H    **
		//               **   B0  ****
		//      *  *      **  [Q ****
		//     *** **    ***********
		//      ******  *********     **
		//      ******  *****HA     ****
		//        *****  ***;AD   *****
		//        ***** *** jN#  ***
		//         **   ****NN5 *** *
		//          *** ****N#** ***
		//           **  **]08*********
		//           ******##8** ****
		//             *** ## *****
		//             ***|&8 * **
		//         ^,E&***lNH****-m&
		//          ^mF***BM **y0*DQ
		//        ,p$&Ep_?MD **K&~~WE,
		//       y0&M 060~GURU`"~&&Q&WI,
		//      ~~'hNH7~ 2&mKk$KwK40Q$~*+
		//         #YmbdB##EMQGW&N6Nx
		//          *N&E&WB08NNH#6r6
		//               ^  ~~""^

	case fd.Type.Kind() != reflect.Pointer &&
		rv.Type().Kind() == reflect.Pointer &&
		rv.Elem().Type().ConvertibleTo(fd.Type):
		into.FieldByIndex(fd.Index).Set(rv.Elem().Convert(fd.Type))

	case rv.Type().ConvertibleTo(fd.Type):
		into.FieldByIndex(fd.Index).Set(rv.Convert(fd.Type))

	case rv.Type().Kind() == reflect.Slice && fd.Type.Kind() == reflect.Slice:
		// rv is []interface, f is specialised. Check if rv[i] can be
		// convertible to f[i]. Bear in mind that slices do not take optional
		// values, so no pointers here.
		ft := fd.Type.Elem()
		for i := 0; i < rv.Len(); i++ {
			if !rv.Index(i).Elem().Type().ConvertibleTo(ft) {
				return false
			}
		}

		// All items can be converted. Initialise a new slice, fill it, and
		// set the field value.
		slice := reflect.MakeSlice(reflect.SliceOf(ft), rv.Len(), rv.Len())
		for i := 0; i < rv.Len(); i++ {
			v := rv.Index(i).Elem()
			slice.Index(i).Set(v.Convert(ft))
		}
		into.FieldByIndex(fd.Index).Set(slice)

	case fd.Type.Kind() == reflect.Bool &&
		(rv.Type().Kind() == reflect.Uint64 || rv.Type().Kind() == reflect.Int64):
		into.FieldByIndex(fd.Index).SetBool(rv.Type().Kind() == reflect.Int64)

	case rv.Type().Kind() == reflect.Pointer &&
		fd.Type.Kind() == reflect.Map &&
		rv.Type() == reflectedMapValue:
		mv := rv.Interface().(*MapValue)
		ok, mi := makeMap(mv, fd.Type)
		if !ok {
			return false
		}
		into.FieldByIndex(fd.Index).Set(mi)

	case rv.Type().Kind() == reflect.Pointer &&
		rv.Type().Elem().Kind() == reflect.Struct &&
		!rv.IsNil() &&
		fd.Type.Kind() == reflect.Struct:
		// rv is a pointer to struct coming from Decode, but we want a concrete
		// value.
		return setValue(into, fd, rv.Elem())

	default:
		return false
	}

	return true
}

func makeMap(v *MapValue, mapType reflect.Type) (bool, reflect.Value) {
	mapKeyType := mapType.Key()
	mapValueType := mapType.Elem()
	var mi reflect.Value
	if v != nil {
		for _, v := range v.Keys {
			if !reflect.TypeOf(v).ConvertibleTo(mapKeyType) {
				return false, reflect.Value{}
			}
		}

		for _, v := range v.Values {
			if !reflect.TypeOf(v).ConvertibleTo(mapValueType) {
				return false, reflect.Value{}
			}
		}
		mi = reflect.MakeMapWithSize(mapType, len(v.Keys))
		for i, rk := range v.Keys {
			k := reflect.ValueOf(rk).Convert(mapKeyType)
			v := reflect.ValueOf(v.Values[i]).Convert(mapValueType)
			mi.SetMapIndex(k, v)
		}
	} else {
		mi = reflect.MakeMapWithSize(mapType, 0)
	}

	return true, mi
}

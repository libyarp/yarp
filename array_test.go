package yarp

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestArrayInts(t *testing.T) {
	items := []uint8{
		0xC0,
		0xFF,
		0xEE,
	}
	encoded, err := encodeArray(reflect.ValueOf(items))
	require.NoError(t, err)
	require.Equal(t, []byte{0x61, 0xc, 0x23, 0x80, 0x23, 0xfe, 0x23, 0xdc}, encoded)
	ty, decoded, err := Decode(bytes.NewReader(encoded))
	require.NoError(t, err)
	assert.Equal(t, Array, ty)
	assert.EqualValues(t, 0xc0, decoded.([]interface{})[0])
	assert.EqualValues(t, 0xff, decoded.([]interface{})[1])
	assert.EqualValues(t, 0xee, decoded.([]interface{})[2])
}

func TestArrayStrings(t *testing.T) {
	items := []string{
		"Coffee",
		"Caffé",
		"Covfefe",
	}
	encoded, err := encodeArray(reflect.ValueOf(items))
	require.NoError(t, err)
	require.Equal(t, []byte{0x61, 0x32, 0xa1, 0xc, 0x43, 0x6f, 0x66, 0x66, 0x65, 0x65, 0xa1, 0xc, 0x43, 0x61, 0x66, 0x66, 0xc3, 0xa9, 0xa1, 0xe, 0x43, 0x6f, 0x76, 0x66, 0x65, 0x66, 0x65}, encoded)
	ty, decoded, err := Decode(bytes.NewReader(encoded))
	require.NoError(t, err)
	assert.Equal(t, Array, ty)
	assert.EqualValues(t, "Coffee", decoded.([]interface{})[0])
	assert.EqualValues(t, "Caffé", decoded.([]interface{})[1])
	assert.EqualValues(t, "Covfefe", decoded.([]interface{})[2])
}

func TestArrayFloat(t *testing.T) {
	items := []float32{
		0.1,
		0.2,
		0.3,
	}
	encoded, err := encodeArray(reflect.ValueOf(items))
	require.NoError(t, err)
	require.Equal(t, []byte{0x61, 0x1e, 0x40, 0xcd, 0xcc, 0xcc, 0x3d, 0x40, 0xcd, 0xcc, 0x4c, 0x3e, 0x40, 0x9a, 0x99, 0x99, 0x3e}, encoded)
	ty, decoded, err := Decode(bytes.NewReader(encoded))
	require.NoError(t, err)
	assert.Equal(t, Array, ty)
	assert.EqualValues(t, 0.1, decoded.([]interface{})[0])
	assert.EqualValues(t, 0.2, decoded.([]interface{})[1])
	assert.EqualValues(t, 0.3, decoded.([]interface{})[2])
}

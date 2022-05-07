package yarp

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestString(t *testing.T) {
	val := "Hello, World!"
	v, err := encode(reflect.ValueOf(val))
	require.NoError(t, err)
	assert.Equal(t, []byte{0xa1, 0x1a, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21}, v)
	ty, s, err := Decode(bytes.NewReader(v))
	require.NoError(t, err)
	assert.Equal(t, String, ty)
	assert.EqualValues(t, "Hello, World!", s)
}

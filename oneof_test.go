package yarp

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOneOf(t *testing.T) {
	v := &OneOfValue{
		Index: 45,
		Data:  "Hello, World!",
	}
	b, err := encodeOneOf(v)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xe1, 0x22, 0x21, 0x5a, 0xa1, 0x1a, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21}, b)
	assert.Equal(t, OneOf, detectType(b[0]))
	ty, oo, err := Decode(bytes.NewReader(b))
	require.NoError(t, err)
	require.Equal(t, OneOf, ty)
	fmt.Printf("%#v\n", oo)
}

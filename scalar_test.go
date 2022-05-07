package yarp

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScalarUint(t *testing.T) {
	for i := 0; i < 1024; i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			buf := encodeUint(uint64(i))
			s, v, err := decodeScalar(buf[0], bytes.NewReader(buf[1:]))
			require.NoError(t, err)
			require.False(t, s)
			assert.Equal(t, Scalar, detectType(buf[0]))
			assert.Equal(t, uint64(i), v, "Buffer data is %#v", buf)
		})
	}
}

func TestScalarInt(t *testing.T) {
	for i := -512; i < 512; i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			buf := encodeInt(int64(i))
			s, v, err := decodeScalar(buf[0], bytes.NewReader(buf[1:]))
			require.NoError(t, err)
			require.True(t, s)
			assert.Equal(t, Scalar, detectType(buf[0]))
			assert.Equal(t, uint64(i), v, "Buffer data is %#v", buf)
		})
	}
}

func TestScalarBool(t *testing.T) {
	buf := encodeBool(true)
	require.Len(t, buf, 1)
	assert.Equal(t, uint8(0x30), buf[0])
	assert.Equal(t, Scalar, detectType(buf[0]))

	buf = encodeBool(false)
	require.Len(t, buf, 1)
	assert.Equal(t, uint8(0x20), buf[0])
	assert.Equal(t, Scalar, detectType(buf[0]))
}

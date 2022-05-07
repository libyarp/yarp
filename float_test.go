package yarp

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestFloat32(t *testing.T) {
	var v float32 = math.Pi
	buf := encodeFloat32(v)
	b, f, err := decodeFloat(buf[0], bytes.NewReader(buf[1:]))
	require.NoError(t, err)
	assert.Equal(t, 32, b)
	assert.InDelta(t, v, f, 1)
}

func TestFloat64(t *testing.T) {
	buf := encodeFloat64(math.Pi)
	b, f, err := decodeFloat(buf[0], bytes.NewReader(buf[1:]))
	require.NoError(t, err)
	assert.Equal(t, 64, b)
	assert.InDelta(t, math.Pi, f, 1)
}

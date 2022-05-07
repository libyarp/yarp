package yarp

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRequest(t *testing.T) {
	req := Request{
		Method: 0x00,
		Headers: map[string]string{
			"RequestID": "Hello!",
		},
	}
	data, err := req.Encode()
	require.NoError(t, err)
	assert.Equal(t, []byte{0x79, 0x79, 0x72, 0x21, 0x34, 0x20, 0xc1, 0x2e, 0x21, 0x16, 0xa1, 0x12, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x44, 0x21, 0x10, 0xa1, 0xc, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x21}, data)

	decoded := Request{}
	err = decoded.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, uint64(0x0), decoded.Method)
	require.Equal(t, "Hello!", decoded.Headers["RequestID"])
}

func TestResponse(t *testing.T) {
	res := Response{
		Headers: map[string]string{
			"Header": "Value",
		},
		Stream: true,
	}
	data, err := res.Encode()
	require.NoError(t, err)
	assert.Equal(t, []byte{0x79, 0x79, 0x52, 0xc1, 0x26, 0x21, 0x10, 0xa1, 0xc, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x21, 0xe, 0xa1, 0xa, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x30}, data)

	decoded := Response{}
	err = decoded.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, true, decoded.Stream)
	require.Equal(t, "Value", decoded.Headers["Header"])
}

func TestError(t *testing.T) {
	res := Error{
		Headers:    map[string]string{"Header": "Value"},
		Identifier: "Identifier",
		UserData:   nil,
	}
	data, err := res.Encode()
	require.NoError(t, err)
	assert.Equal(t, []byte{0x79, 0x79, 0x65, 0x20, 0xc1, 0x26, 0x21, 0x10, 0xa1, 0xc, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x21, 0xe, 0xa1, 0xa, 0x56, 0x61, 0x6c, 0x75, 0x65, 0xa1, 0x14, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x65, 0x72, 0xc0}, data)

	decoded := Error{}
	err = decoded.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "Value", decoded.Headers["Header"])
	require.Equal(t, "Identifier", decoded.Identifier)
	require.Empty(t, decoded.UserData)
}

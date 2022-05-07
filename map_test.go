package yarp

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestMap(t *testing.T) {
	val := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
		"d": 4,
	}
	data, err := encode(reflect.ValueOf(val))
	require.NoError(t, err)
	// Can't be tested through []byte, since Go's map order is non-deterministic.
	//assert.Equal(t, []byte{0xc1, 0x22, 0x21, 0x10, 0xa2, 0x61, 0xa2, 0x62, 0xa2, 0x63, 0xa2, 0x64, 0x21, 0xa, 0x32, 0x34, 0x36, 0x31, 0x8}, data)
	assert.Equal(t, Map, detectType(data[0]))
	dec, err := decodeMap(data[0], bytes.NewReader(data[1:]))
	require.NoError(t, err)
	for k, v := range val {
		kOk, vOk := false, false
		for _, kV := range dec.Keys {
			if kV == k {
				kOk = true
			}
		}

		for _, vV := range dec.Values {
			if int(vV.(int64)) == v {
				vOk = true
			}
		}

		assert.True(t, kOk, "%#v should be present in %#v", k, dec.Keys)
		assert.True(t, vOk, "%#v should be present in %#v", v, dec.Values)
	}
}

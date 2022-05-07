package yarp

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

type OtherTS struct {
	*Structure
	Project string `index:"0"`
	Role    string `index:"1"`
}

func (T OtherTS) YarpID() uint64         { return 0x2 }
func (T OtherTS) YarpPackage() string    { return "io.vito" }
func (T OtherTS) YarpStructName() string { return "TS2" }

type TS struct {
	*Structure
	ID          int            `index:"0"`
	Name        string         `index:"1"`
	Email       string         `index:"2"`
	Keys        []string       `index:"3"`
	Other       []OtherTS      `index:"4"`
	AMap        map[string]int `index:"5"`
	OneOfA      *string        `index:"6,0"`
	HasOneOfA   bool
	OneOfB      *int `index:"6,1"`
	HasOneOfB   bool
	OneOfC      *bool `index:"6,2"`
	HasOneOfC   bool
	IsAdmin     bool     `index:"7"`
	SingleOther OtherTS  `index:"8"`
	OptionalTS  *OtherTS `index:"9"`
}

func (T TS) YarpID() uint64         { return 0x1 }
func (T TS) YarpPackage() string    { return "io.vito" }
func (T TS) YarpStructName() string { return "TS" }

func TestStruct(t *testing.T) {
	t.Cleanup(resetRegistry)
	RegisterStructType(TS{}, OtherTS{})
	strValue := "test"
	v := TS{
		ID:    102030,
		Name:  "Vito",
		Email: "hey@vito.io",
		Keys:  []string{"a", "b", "c"},
		Other: []OtherTS{
			{
				Project: "Foo",
				Role:    "Bar",
			},
		},
		AMap: map[string]int{
			"a": 1,
			"b": 2,
			"c": 3,
			"d": 4,
		},
		OneOfA:  &strValue,
		IsAdmin: true,
		SingleOther: OtherTS{
			Project: "Fuz",
			Role:    "Baz",
		},
	}
	data, err := encode(reflect.ValueOf(v))
	require.NoError(t, err)
	fmt.Printf("\n%s\n", hex.Dump(data))
	//assert.Equal(t, []byte{0x81, 0x4e, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x31, 0xd, 0x3b, 0x1c, 0xa1, 0x8, 0x56, 0x69, 0x74, 0x6f, 0xa1, 0x16, 0x68, 0x65, 0x79, 0x40, 0x76, 0x69, 0x74, 0x6f, 0x2e, 0x69, 0x6f, 0x61, 0xc, 0xa2, 0x61, 0xa2, 0x62, 0xa2, 0x63}, data)
	assert.Equal(t, Struct, detectType(data[0]))
	str, err := decodeStruct(data[0], bytes.NewReader(data[1:]))
	require.NoError(t, err)
	fmt.Printf("%#v\n", str)
	ty, decodedStr, err := Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, Struct, ty)
	assert.NotNil(t, decodedStr)
	fmt.Printf("%#v\n", decodedStr)

	ss := decodedStr.(*TS)
	assert.Equal(t, 102030, ss.ID)
	assert.Equal(t, "Vito", ss.Name)
	assert.Equal(t, "hey@vito.io", ss.Email)
	assert.EqualValues(t, []string{"a", "b", "c"}, ss.Keys)
	assert.Equal(t, "Foo", ss.Other[0].Project)
	assert.Equal(t, "Bar", ss.Other[0].Role)
	assert.Equal(t, 1, ss.AMap["a"])
	assert.Equal(t, 2, ss.AMap["b"])
	assert.Equal(t, 3, ss.AMap["c"])
	assert.Equal(t, 4, ss.AMap["d"])
	assert.Equal(t, "test", *ss.OneOfA)
	assert.Equal(t, true, ss.IsAdmin)
	assert.Equal(t, "Fuz", ss.SingleOther.Project)
	assert.Equal(t, "Baz", ss.SingleOther.Role)
	assert.Nil(t, ss.OptionalTS)
}

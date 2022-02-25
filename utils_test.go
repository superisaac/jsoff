package jsonz

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

type vt1 struct {
	Username string `json:"user_name"`
	Age      int    `json:"a"`
}

func TestDecode(t *testing.T) {
	assert := assert.New(t)

	var v vt1
	err := DecodeInterface(map[string]interface{}{
		"user_name": "boy",
		"a":         5,
	}, &v)

	assert.Nil(err)
	assert.Equal("boy", v.Username)
	assert.Equal(5, v.Age)
}

type op struct {
	H        string
	privateI string
	J        int
}

type tst struct {
	A        int
	privateA string
	B        *string
	C        op
	D        string
}

func TestDecodeParams(t *testing.T) {
	assert := assert.New(t)

	params := [](interface{}){
		109,
		"hello",
		map[string](interface{}){
			"h": "hidden",
			"j": 688}}

	output := tst{}
	err := DecodeParams(params, &output)
	assert.Nil(err)
	assert.Equal(109, output.A)
	assert.Equal("", output.privateA)
	assert.Equal("hello", *output.B)
	assert.Equal("hidden", output.C.H)
	assert.Equal("", output.C.privateI)
	assert.Equal(688, output.C.J)
	assert.Equal("", output.D)

	params1 := []interface{}{}
	output1 := 999
	err1 := DecodeParams(params1, &output1)
	assert.Equal("output is not pointer of struct", err1.Error())

	params2 := [](interface{}){"hello"}
	output2 := tst{}
	err2 := DecodeParams(params2, &output2)
	assert.NotNil(err2)
	assert.True(strings.Contains(err2.Error(), "expected type 'int'"))
}

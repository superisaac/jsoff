package jlib

import (
	"encoding/json"
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

func TestGuessJson(t *testing.T) {
	assert := assert.New(t)

	v1, err := GuessJson("")
	assert.Nil(err)
	assert.Equal("", v1)

	v1_0, err := GuessJson("5")
	assert.Equal(int64(5), v1_0)

	v1_1, err := GuessJson("-5")
	assert.Equal(int64(-5), v1_1)

	v1_2, err := GuessJson("-5.78389383")
	assert.InDelta(float64(-5.78389383), v1_2, 0.0001)

	v2, err := GuessJson("false")
	assert.Equal(false, v2)

	_, err = GuessJson("[aaa")
	assert.Contains(err.Error(), "invalid character")

	_, err = GuessJson("{aaa")
	assert.Contains(err.Error(), "invalid character")

	v3, err := GuessJson(`{"abc": 5}`)
	map3 := v3.(map[string]interface{})
	assert.NotNil(map3)
	assert.Equal(json.Number("5"), map3["abc"])

	v4, err := GuessJsonArray([]string{"5", "hahah", `{"ccc": 6}`})
	assert.Equal(3, len(v4))
	assert.Equal(int64(5), v4[0])
	assert.Equal("hahah", v4[1])

	v5, err := GuessJson(`["abc", 666.99, {"kic": 5}]`)
	arr5 := v5.([]interface{})
	assert.Equal(3, len(arr5))
	assert.Equal("abc", arr5[0])
	assert.Equal(json.Number("666.99"), arr5[1])

	v6, err := GuessJson(`"666"`)
	s6 := v6.(string)
	assert.Equal("666", s6)

	v7, err := GuessJson(`"-666.999"`)
	s7 := v7.(string)
	assert.Equal("-666.999", s7)

	v8, err := GuessJson(`@testdata/guess.json`)
	assert.Nil(err)
	m8 := v8.(map[string]interface{})
	assert.Equal("ttt", m8["aaa"])
	assert.Equal(json.Number("789"), m8["bbb"])

	_, err = GuessJson(`@testdata/nosuchfile.json`)
	assert.Equal("open testdata/nosuchfile.json: no such file or directory", err.Error())

}

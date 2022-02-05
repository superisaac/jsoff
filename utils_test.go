package jsonz

import (
	"github.com/stretchr/testify/assert"
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

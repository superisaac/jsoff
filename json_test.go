package jsoff

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMarshalBigint(t *testing.T) {
	assert := assert.New(t)

	type vwrap struct {
		V *Bigint `json:"v"`
	}

	a := vwrap{
		V: &Bigint{},
	}
	a.V.Load("1234498219282917838937829383759200002030081000698")

	data, err := json.Marshal(&a)
	assert.Nil(err)
	assert.Equal(`{"v":"1234498219282917838937829383759200002030081000698"}`, string(data))

	var b vwrap
	err = json.Unmarshal(data, &b)
	assert.Nil(err)
	assert.Equal(a.V.Value(), b.V.Value())
	assert.Equal(a.V, b.V)
	assert.Equal("1234498219282917838937829383759200002030081000698", b.V.String())

	strnoquote := `{"v": 1234498219282917838937829383759200002030081000698}`
	var c vwrap
	err = json.Unmarshal([]byte(strnoquote), &c)
	assert.Nil(err)
	assert.Equal(a.V.Value(), c.V.Value())
	assert.Equal(a.V, c.V)
	assert.Equal("1234498219282917838937829383759200002030081000698", c.V.String())

}

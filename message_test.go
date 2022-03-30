package jsonz

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestValidators(t *testing.T) {
	assert := assert.New(t)

	assert.True(IsMethod(".abc+def"))
	assert.False(IsPublicMethod(".abc+def"))
	assert.True(IsPublicMethod("textDocument/didOpen"))
	assert.True(IsPublicMethod("namespace::show"))
	assert.True(IsPublicMethod("namespace#show"))
}

func TestParseParams(t *testing.T) {
	assert := assert.New(t)

	j0 := `
{"text": "what"}
`
	_, err := ParseBytes([]byte(j0))
	assert.Equal("error decode: not a jsonrpc message", err.Error())

	j1 := `{
"id": 99,
"method": "abc::def",
"params": {"what": "hello"}
}`
	msg, err := ParseBytes([]byte(j1))
	assert.Nil(err)

	params := msg.MustParams()
	assert.Equal(1, len(params))
	mapp, _ := params[0].(map[string]interface{})
	assert.Equal("hello", mapp["what"])
	assert.True(msg.IsRequest())
	reqmsg, _ := msg.(*RequestMessage)
	assert.False(reqmsg.paramsAreList)

	j2 := `{
"id": "stringid",
"method": "abc::def"
}`

	_, err = ParseBytes([]byte(j2))
	assert.NotNil(err)
	assert.Equal("error decode: no params field", err.Error())

	// empty
	j3 := `{
"id": "stringid",
"method": "abc::def",
"params": []
}`
	reqmsg3, err := ParseBytes([]byte(j3))
	assert.Nil(err)
	assert.True(reqmsg3.IsRequest())
	assert.Equal("stringid", reqmsg3.MustId())
	assert.Equal(0, len(reqmsg3.MustParams()))
}

func TestRequestMsg(t *testing.T) {
	assert := assert.New(t)

	j1 := `{
  "id": 100,
  "method": "abc::add",
  "params": [3, 4, 5]
  }`

	msg, err := ParseBytes([]byte(j1))
	assert.Nil(err)

	assert.True(msg.IsRequest())
	assert.False(msg.IsNotify())
	assert.False(msg.IsError())
	assert.False(msg.IsResult())

	assert.Equal(100, msg.MustId())
	assert.Equal("abc::add", msg.MustMethod())

	rpcErr := ParamsError("user issued")
	assert.Equal(-32602, rpcErr.Code)
	assert.Equal("user issued", rpcErr.Message)
}

func TestNotifyMsg(t *testing.T) {
	assert := assert.New(t)

	j1 := `{
  "method": "abc::add",
  "params": [13, 4, "hello"]
  }`

	msg, err := ParseBytes([]byte(j1))
	assert.Nil(err)

	assert.Equal("abc::add", msg.MustMethod())

	assert.False(msg.IsRequest())
	assert.True(msg.IsNotify())
	assert.False(msg.IsError())
	assert.False(msg.IsResult())

	params := msg.MustParams()
	assert.Equal(len(params), 3)
	assert.Equal(params[0], json.Number("13"))
	assert.Equal(params[1], json.Number("4"))
	assert.Equal(params[2], "hello")

	arr := [](interface{}){3, "uu"}
	msg = NewNotifyMessage("hahaha", arr)
	assert.Equal("hahaha", msg.MustMethod())
	params = msg.MustParams()

	assert.Equal(len(params), 2)
	assert.Equal(params[1], "uu")
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
}

func TestDecodeMessage(t *testing.T) {
	assert := assert.New(t)

	input := `
{"id": 1, "method": "is_request", "params": ["hello", 2]}

{"method": "is_notify",
"params": {"name": "ok"}}

{"error": {"code": -300, "message": "error message", "data": "bad message"}, "id": 3}

{"result": 66.89, "id": 999}
{"result": "result without id"}

`
	dec := json.NewDecoder(strings.NewReader(input))
	dec.UseNumber()

	msg1, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg1.IsRequest())
	assert.Equal("is_request", msg1.MustMethod())
	assert.Equal("hello", msg1.MustParams()[0])
	assert.Equal(json.Number("2"), msg1.MustParams()[1])

	msg2, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg2.IsNotify())
	assert.Equal("is_notify", msg2.MustMethod())
	assert.Equal(map[string](interface{}){"name": "ok"}, msg2.MustParams()[0])

	msg3, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg3.IsError())
	assert.Equal(-300, msg3.MustError().Code)
	assert.Equal("error message", msg3.MustError().Message)
	assert.Equal("bad message", msg3.MustError().Data)

	msg4, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg4.IsResult())
	assert.Equal(json.Number("66.89"), msg4.MustResult())

	msg5, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg5.IsResult())
	assert.Nil(msg5.MustId())

	// EOF expected
	_, err = DecodeMessage(dec)
	assert.Equal(io.EOF, err)
}

func TestDecodePipeBytes(t *testing.T) {
	assert := assert.New(t)

	r, w := io.Pipe()
	dec := json.NewDecoder(r)

	go func() {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{"method": "hello",`))
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`"params": [3.09]}`))
	}()

	msg, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg.IsNotify())
	assert.Equal(json.Number("3.09"), msg.MustParams()[0])
}

package jsoff

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestValidators(t *testing.T) {
	assert := assert.New(t)

	assert.True(IsMethod(".abc+def"))
	assert.False(IsPublicMethod("rpc.list"))
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

func TestParseNullBytes(t *testing.T) {
	assert := assert.New(t)

	data := `{"jsonrpc":"2.0","id":1,"result":null}`
	_, err := ParseBytes([]byte(data))
	assert.Nil(err)
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

func TestDecodeMessage(t *testing.T) {
	assert := assert.New(t)

	input := strings.Join([]string{
		`{"id": 1, "method": "is_request", "params": ["hello", 2]}`, // test normal request
		`{"method": "is_notify",
	"params": {"name": "ok"}}

	`, // test multi line notify
		`{"error": {"code": -300, "message": "error message", "data": "bad message"}, "id": 3}

	`, // test bad message

		`{"result": 66.89, "id": 999}`, // test result

		`{"result": "result without id"}`, // test result with empty id, take empty id as null id

		`{"id": null, "method": "is_request", "params": ["hello", 3]}`, // test request with null id

		`{"result": 99.992, "id": null}`, // test result with null id

		`{"error": {"code": -32600, "message": "invalid request", "data": "bad message"}, "id": null}`, // test error with null id

		`{"id": {"what": "not a plain id"}, "method": "is_request", "params": ["hello", 3]}`, // test request with non plain id

		`{"result": null, "error": {"code": -27, "message": "X bluh"}, "id": 1}`, // test error message with result null
	}, "\n")

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

	msg6, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg6.IsRequest())
	assert.Nil(msg6.MustId())

	msg7, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg7.IsResult())
	assert.Equal(json.Number("99.992"), msg7.MustResult())
	assert.Nil(msg7.MustId())

	msg8, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg8.IsError())
	assert.Nil(msg8.MustId())
	assert.Equal(-32600, msg8.MustError().Code)

	_, err9 := DecodeMessage(dec)
	assert.NotNil(err9)
	assert.Contains(err9.Error(), "cannot unmarshal object into Go struct field msgUnion.id of type string")

	msg10, err := DecodeMessage(dec)
	assert.Nil(err)
	assert.True(msg10.IsError())
	assert.False(msg10.IsResult())
	assert.Equal(-27, msg10.MustError().Code)

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

func TestMessages(t *testing.T) {
	assert := assert.New(t)

	reqmsg := NewRequestMessage(101, "queryMethod", []interface{}{"p1", "p2"})
	assert.Equal(101, reqmsg.Id)
	assert.Equal("queryMethod", reqmsg.Method)
	assert.Equal(2, len(reqmsg.Params))

	msgtype, _ := reqmsg.Log().Data["msgtype"]
	assert.Equal("request", msgtype)

	reqmap, _ := MessageMap(reqmsg)
	m1, _ := reqmap["method"]
	assert.Equal("queryMethod", m1)

	reqmsg1 := reqmsg.ReplaceId(102)
	assert.True(reqmsg1.IsResponse())
	assert.False(reqmsg1.IsResultOrError())

	assert.Equal(101, reqmsg.Id)
	assert.Equal(102, reqmsg1.MustId())
	assert.Equal("p1", reqmsg1.MustParams()[0])
	assert.Equal("p2", reqmsg1.MustParams()[1])
	reqmsg.Params = append(reqmsg.Params, "p3")
	assert.Equal(2, len(reqmsg1.MustParams()))
	assert.Equal(3, len(reqmsg.Params))

	resmsg := NewResultMessage(reqmsg, "p1p2")
	assert.Equal(101, resmsg.Id)
	resmap, _ := MessageMap(resmsg)
	resid, _ := resmap["id"]
	assert.Equal(101, resid)
	resmsg1 := resmsg.ReplaceId(104)
	assert.True(resmsg1.IsResult())
	assert.Equal(104, resmsg1.MustId())

	msgtype, _ = resmsg.Log().Data["msgtype"]
	assert.Equal("result", msgtype)

	errmsg := NewErrorMessage(reqmsg, ParamsError("p error"))
	assert.Equal(101, errmsg.Id)
	errmap, _ := MessageMap(errmsg)
	errid, _ := errmap["id"]
	assert.Equal(101, errid)
	errmsg1 := errmsg.ReplaceId(103)
	assert.True(errmsg1.IsError())
	assert.Equal(103, errmsg1.MustId())

	msgtype, _ = errmsg.Log().Data["msgtype"]
	assert.Equal("error", msgtype)

	ntfmsg := NewNotifyMessage("queryReceived", nil)
	assert.Equal("queryReceived", ntfmsg.Method)

	msgtype, _ = ntfmsg.Log().Data["msgtype"]
	assert.Equal("notify", msgtype)

	ntfmap, _ := MessageMap(ntfmsg)
	m2, _ := ntfmap["method"]
	assert.Equal("queryReceived", m2)

	assert.False(NewRequestMessage(800, "aaa", map[string]interface{}{}).paramsAreList)
	assert.True(NewRequestMessage(800, "aaa", nil).paramsAreList)

	assert.False(NewNotifyMessage("aaa", map[string]interface{}{"aa": "bb"}).paramsAreList)
	assert.True(NewNotifyMessage("aaa", nil).paramsAreList)

}

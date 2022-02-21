package jsonzhttp

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsonz"
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

const (
	addSchema = `{
  "type": "method",
  "params": ["integer", {"type": "integer"}]
}`
)

func TestServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewServer()
	server.Handler.On("echo", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsonz.ParamsError("no argument given")
		}
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28000", server)
	time.Sleep(10 * time.Millisecond)

	client := NewHTTPClient("http://127.0.0.1:28000")

	// right request
	params := [](interface{}){"hello999"}
	reqmsg := jsonz.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello999", res)

	// method not found
	params1 := [](interface{}){"hello999"}
	reqmsg1 := jsonz.NewRequestMessage(666, "echoxxx", params1)
	resmsg1, err := client.Call(rootCtx, reqmsg1)
	assert.Nil(err)
	assert.True(resmsg1.IsError())
	errbody := resmsg1.MustError()
	assert.Equal(jsonz.ErrMethodNotFound.Code, errbody.Code)
}

func TestMissing(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewServer()
	err := server.Handler.OnMissing(func(req *RPCRequest) (interface{}, error) {
		msg := req.Msg()
		assert.True(msg.IsNotify())
		assert.Equal("testnotify", msg.MustMethod())
		return nil, nil
	})
	assert.Nil(err)

	go ListenAndServe(rootCtx, "127.0.0.1:28003", server)
	time.Sleep(10 * time.Millisecond)

	client := NewHTTPClient("http://127.0.0.1:28003")
	// right request
	params := [](interface{}){"hello999"}
	ntfmsg := jsonz.NewNotifyMessage("testnotify", params)

	err = client.Send(rootCtx, ntfmsg)
	assert.Nil(err)
}

func TestTypedServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewServer()
	err := server.Handler.OnTyped("echoTyped", func(req *RPCRequest, v string) (string, error) {
		return v, nil
	})
	assert.Nil(err)

	err = server.Handler.OnTyped("add", func(req *RPCRequest, a, b int) (int, error) {
		return a + b, nil
	})
	assert.Nil(err)

	go ListenAndServe(rootCtx, "127.0.0.1:28001", server)
	time.Sleep(10 * time.Millisecond)

	client := NewHTTPClient("http://127.0.0.1:28001")

	// right request
	params := [](interface{}){"hello999"}
	reqmsg := jsonz.NewRequestMessage(1, "echoTyped", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello999", res)

	// type mismatch
	params1 := [](interface{}){true}
	reqmsg1 := jsonz.NewRequestMessage(1, "echoTyped", params1)

	resmsg1, err1 := client.Call(rootCtx, reqmsg1)
	assert.Nil(err1)
	assert.True(resmsg1.IsError())
	errbody1 := resmsg1.MustError()
	assert.Equal(-32602, errbody1.Code) // params error
	assert.True(strings.Contains(errbody1.Message, "got unconvertible type"))
	// test params size
	params2 := [](interface{}){"hello", 2}
	reqmsg2 := jsonz.NewRequestMessage(2, "echoTyped", params2)

	resmsg2, err2 := client.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.True(resmsg2.IsError())
	errbody2 := resmsg2.MustError()
	assert.Equal(-32602, errbody2.Code)
	assert.Equal("different params size", errbody2.Message)

	// test add 2 numbers
	params3 := [](interface{}){6, 3}
	reqmsg3 := jsonz.NewRequestMessage(3, "add", params3)
	resmsg3, err3 := client.Call(rootCtx, reqmsg3)
	assert.Nil(err3)
	assert.True(resmsg3.IsResult())
	res3 := resmsg3.MustResult()
	assert.Equal(json.Number("9"), res3)

	// test add 2 numbers with typing mismatch
	params4 := [](interface{}){"6", 4}
	reqmsg4 := jsonz.NewRequestMessage(4, "add", params4)
	resmsg4, err4 := client.Call(rootCtx, reqmsg4)
	assert.Nil(err4)
	assert.True(resmsg4.IsError())
	errbody4 := resmsg4.MustError()
	assert.Equal(-32602, errbody4.Code)
	assert.True(strings.Contains(errbody4.Message, "got unconvertible type"))

	// test add 2 numbers with typing mismatch
	params5 := [](interface{}){"6", 5}
	reqmsg5 := jsonz.NewRequestMessage(5, "add", params5)
	var res5 int
	err5 := client.UnwrapCall(rootCtx, reqmsg5, &res5)
	assert.NotNil(err5)
	var errbody5 *jsonz.RPCError
	assert.True(errors.As(err5, &errbody5))
	assert.Equal(-32602, errbody5.Code)
	assert.True(strings.Contains(errbody5.Message, "got unconvertible type"))

}

func TestHandlerSchema(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewServer()
	server.Handler.VerifySchema = true
	server.Handler.On("add2num", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		a := jsonz.ConvertInt(params[0])
		b := jsonz.ConvertInt(params[1])
		return a + b, nil
	}, WithSchemaJson(addSchema))

	go ListenAndServe(rootCtx, "127.0.0.1:28040", server)
	time.Sleep(10 * time.Millisecond)

	client := NewHTTPClient("http://127.0.0.1:28040")

	// right request
	reqmsg := jsonz.NewRequestMessage(
		1, "add2num", []interface{}{5, 8})
	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.Equal(json.Number("13"), resmsg.MustResult())

	reqmsg2 := jsonz.NewRequestMessage(
		2, "add2num", []interface{}{"12", "a str"})
	resmsg2, err2 := client.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.Equal(jsonz.ErrInvalidSchema.Code, resmsg2.MustError().Code)
	assert.Equal("Validation Error: .params[0] data is not integer", resmsg2.MustError().Message)
}

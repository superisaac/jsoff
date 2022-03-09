package jsonzhttp

import (
	"context"
	"encoding/json"
	//log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsonz"
	"strings"
	"testing"
	"time"
)

func TestWSHandlerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewWSHandler(rootCtx, nil)
	server.Actor.On("echo", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsonz.ParamsError("no argument given")
		}
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28100", server)
	time.Sleep(10 * time.Millisecond)

	client := NewWSClient(urlParse("ws://127.0.0.1:28100"))

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

func TestTypedWSHandlerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewWSHandler(rootCtx, nil)
	err := server.Actor.OnTyped("echoTyped", func(req *RPCRequest, v string) (string, error) {
		return v, nil
	})
	assert.Nil(err)

	err = server.Actor.OnTyped("add", func(req *RPCRequest, a, b int) (int, error) {
		return a + b, nil
	})
	assert.Nil(err)

	go ListenAndServe(rootCtx, "127.0.0.1:28101", server)
	time.Sleep(10 * time.Millisecond)

	client := NewWSClient(urlParse("ws://127.0.0.1:28101"))

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
}

func TestWSClose(t *testing.T) {
	assert := assert.New(t)

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	clientCtx, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()

	server := NewWSHandler(serverCtx, nil)
	server.Actor.On("echo", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsonz.ParamsError("no argument given")
		}
	})

	go ListenAndServe(serverCtx, "127.0.0.1:28123", server)
	time.Sleep(100 * time.Millisecond)

	closeCalled := make(map[int]bool)
	client := NewWSClient(urlParse("ws://127.0.0.1:28123"))
	client.OnClose(func() {
		closeCalled[0] = true
	})
	// right request
	params := [](interface{}){"hello999"}
	reqmsg := jsonz.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(clientCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello999", res)

	// cancel root
	cancelServer()
	time.Sleep(100 * time.Millisecond)
	assert.True(closeCalled[0])
}

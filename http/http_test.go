package jsonrpchttp

import (
	"context"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsonrpc"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestServerClient(t *testing.T) {
	assert := assert.New(t)

	server := NewServer(nil)
	server.Router.On("echo", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsonrpc.ParamsError("no argument given")
		}
	})

	go http.ListenAndServe("127.0.0.1:28000", server)
	time.Sleep(100 * time.Millisecond)

	client := NewClient("http://127.0.0.1:28000")

	// right request
	params := [](interface{}){"hello999"}
	reqmsg := jsonrpc.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(context.Background(), reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello999", res)

	// method not found
	params1 := [](interface{}){"hello999"}
	reqmsg1 := jsonrpc.NewRequestMessage(666, "echoxxx", params1)
	resmsg1, err := client.Call(context.Background(), reqmsg1)
	assert.Nil(err)
	assert.True(resmsg1.IsError())
	errbody := resmsg1.MustError()
	assert.Equal(jsonrpc.ErrMethodNotFound.Code, errbody.Code)
}

func TestMissing(t *testing.T) {
	assert := assert.New(t)

	server := NewServer(nil)
	err := server.Router.OnMissing(func(req *RPCRequest) (interface{}, error) {
		msg := req.Msg()
		assert.True(msg.IsNotify())
		assert.Equal("testnotify", msg.MustMethod())
		return nil, nil
	})
	assert.Nil(err)

	go http.ListenAndServe("127.0.0.1:28003", server)
	time.Sleep(100 * time.Millisecond)

	client := NewClient("http://127.0.0.1:28003")
	// right request
	params := [](interface{}){"hello999"}
	ntfmsg := jsonrpc.NewNotifyMessage("testnotify", params)

	err = client.Send(context.Background(), ntfmsg)
	assert.Nil(err)
}

func TestTypedServerClient(t *testing.T) {
	assert := assert.New(t)

	server := NewServer(nil)
	err := server.Router.OnTyped("echoTyped", func(req *RPCRequest, v string) (string, error) {
		return v, nil
	})
	assert.Nil(err)

	err = server.Router.OnTyped("add", func(req *RPCRequest, a, b int) (int, error) {
		return a + b, nil
	})
	assert.Nil(err)

	go http.ListenAndServe("127.0.0.1:28001", server)
	time.Sleep(100 * time.Millisecond)

	client := NewClient("http://127.0.0.1:28001")

	// right request
	params := [](interface{}){"hello999"}
	reqmsg := jsonrpc.NewRequestMessage(1, "echoTyped", params)

	resmsg, err := client.Call(context.Background(), reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello999", res)

	// type mismatch
	params1 := [](interface{}){true}
	reqmsg1 := jsonrpc.NewRequestMessage(1, "echoTyped", params1)

	resmsg1, err1 := client.Call(context.Background(), reqmsg1)
	assert.Nil(err1)
	assert.True(resmsg1.IsError())
	errbody1 := resmsg1.MustError()
	assert.Equal(-32602, errbody1.Code) // params error
	assert.True(strings.Contains(errbody1.Message, "got unconvertible type"))
	// test params size
	params2 := [](interface{}){"hello", 2}
	reqmsg2 := jsonrpc.NewRequestMessage(2, "echoTyped", params2)

	resmsg2, err2 := client.Call(context.Background(), reqmsg2)
	assert.Nil(err2)
	assert.True(resmsg2.IsError())
	errbody2 := resmsg2.MustError()
	assert.Equal(-32602, errbody2.Code)
	assert.Equal("different params size", errbody2.Message)

	// test add 2 numbers
	params3 := [](interface{}){6, 3}
	reqmsg3 := jsonrpc.NewRequestMessage(3, "add", params3)
	resmsg3, err3 := client.Call(context.Background(), reqmsg3)
	assert.Nil(err3)
	assert.True(resmsg3.IsResult())
	res3 := resmsg3.MustResult()
	assert.Equal(json.Number("9"), res3)

	// test add 2 numbers with typing mismatch
	params4 := [](interface{}){"6", 4}
	reqmsg4 := jsonrpc.NewRequestMessage(4, "add", params4)
	resmsg4, err4 := client.Call(context.Background(), reqmsg4)
	assert.Nil(err4)
	assert.True(resmsg4.IsError())
	errbody4 := resmsg4.MustError()
	assert.Equal(-32602, errbody4.Code)
	assert.True(strings.Contains(errbody4.Message, "got unconvertible type"))
}

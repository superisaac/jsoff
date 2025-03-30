package jsoffnet

import (
	"context"
	"encoding/json"
	//log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsoff"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWSHandlerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewWSHandler(rootCtx, nil)
	server.Actor.On("echo", func(params []any) (any, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})
	// an auth handler with nil configs
	auth := NewAuthHandler(nil, server)
	go ListenAndServe(rootCtx, "127.0.0.1:28100", auth)
	time.Sleep(10 * time.Millisecond)

	client := NewWSClient(urlParse("ws://127.0.0.1:28100"))
	client.SetExtraHeader(http.Header{"X-Input": []string{"hello"}})
	assert.Equal("ws", client.ServerURL().Scheme)

	erronmessage := client.OnMessage(func(msg jsoff.Message) {
	})
	assert.Nil(erronmessage)

	erronmessage = client.OnMessage(func(msg jsoff.Message) {
	})
	assert.NotNil(erronmessage)
	assert.Contains(erronmessage.Error(), "message handler already exist!")

	// right request
	params := []any{"hello2002"}
	reqmsg := jsoff.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello2002", res)

	// method not found
	params1 := []any{"hello2003"}
	reqmsg1 := jsoff.NewRequestMessage(666, "echoxxx", params1)
	resmsg1, err := client.Call(rootCtx, reqmsg1)
	assert.Nil(err)
	assert.True(resmsg1.IsError())
	errbody1 := resmsg1.MustError()
	assert.Equal(jsoff.ErrMethodNotFound.Code, errbody1.Code)

	// unwrap call
	params2 := []any{"hello2004"}
	reqmsg2 := jsoff.NewRequestMessage(777, "echo", params2)
	var res2 string
	err2 := client.UnwrapCall(rootCtx, reqmsg2, &res2)
	assert.Nil(err2)
	assert.Equal("hello2004", res2)
}

func TestTypedWSHandlerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewWSHandler(rootCtx, nil)
	server.Actor.OnTyped("echoTyped", func(v string) (string, error) {
		return v, nil
	})

	server.Actor.OnTyped("add", func(a, b int) (int, error) {
		return a + b, nil
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28101", server)
	time.Sleep(10 * time.Millisecond)

	client := NewWSClient(urlParse("ws://127.0.0.1:28101"))

	// right request
	params := []any{"hello2005"}
	reqmsg := jsoff.NewRequestMessage(1, "echoTyped", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello2005", res)

	// type mismatch
	params1 := []any{true}
	reqmsg1 := jsoff.NewRequestMessage(1, "echoTyped", params1)

	resmsg1, err1 := client.Call(rootCtx, reqmsg1)
	assert.Nil(err1)
	assert.True(resmsg1.IsError())
	errbody1 := resmsg1.MustError()
	assert.Equal(-32602, errbody1.Code) // params error
	assert.True(strings.Contains(errbody1.Message, "got unconvertible type"))
	// test params size
	params2 := []any{}
	reqmsg2 := jsoff.NewRequestMessage(2, "echoTyped", params2)

	resmsg2, err2 := client.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.True(resmsg2.IsError())
	errbody2 := resmsg2.MustError()
	assert.Equal(-32602, errbody2.Code)
	assert.Equal("no enough params size", errbody2.Message)

	// test add 2 numbers
	params3 := []any{6, 3}
	reqmsg3 := jsoff.NewRequestMessage(3, "add", params3)
	resmsg3, err3 := client.Call(rootCtx, reqmsg3)
	assert.Nil(err3)
	assert.True(resmsg3.IsResult())
	res3 := resmsg3.MustResult()
	assert.Equal(json.Number("9"), res3)

	// test add 2 numbers with typing mismatch
	params4 := []any{"6", 4}
	reqmsg4 := jsoff.NewRequestMessage(4, "add", params4)
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
	server.Actor.On("echo", func(params []any) (any, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
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
	params := []any{"hello2001"}
	reqmsg := jsoff.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(clientCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello2001", res)

	// cancel root
	cancelServer()
	time.Sleep(100 * time.Millisecond)
	assert.True(closeCalled[0])
}

package jsoffhttp

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsoff"
	"testing"
	"time"
)

func TestH2HandlerClient(t *testing.T) {
	assert := assert.New(t)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH2Handler(rootCtx, nil)

	server.Actor.On("echo", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})

	server.Actor.OnMissing(func(req *RPCRequest) (interface{}, error) {
		return nil, nil
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28700", server, serverTLS())
	time.Sleep(10 * time.Millisecond)

	client := NewH2Client(urlParse("h2://127.0.0.1:28700"))
	client.SetClientTLSConfig(clientTLS())
	// right request
	params := [](interface{}){"hello1003"}
	reqmsg := jsoff.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello1003", res)
}

func TestH2CServerClient(t *testing.T) {
	assert := assert.New(t)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH2Handler(rootCtx, nil)

	server.Actor.On("echo", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})

	server.Actor.OnMissing(func(req *RPCRequest) (interface{}, error) {
		return nil, nil
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28800", server.H2CHandler(), nil)
	time.Sleep(10 * time.Millisecond)

	client := NewH2Client(urlParse("h2c://127.0.0.1:28800"))
	assert.True(client.UseH2C)
	//client.SetClientTLSConfig(clientTLS())
	// right request
	params := [](interface{}){"hello1000"}
	reqmsg := jsoff.NewRequestMessage(2, "echo", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello1000", res)
}

func TestTypedH2HandlerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH2Handler(rootCtx, nil)
	server.Actor.OnTyped("echoTyped", func(v string) (string, error) {
		return v, nil
	})

	server.Actor.OnTyped("add", func(a, b int) (int, error) {
		return a + b, nil
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28701", server, serverTLS())
	time.Sleep(10 * time.Millisecond)

	client := NewH2Client(urlParse("h2://127.0.0.1:28701"))
	client.SetClientTLSConfig(clientTLS())

	// right request
	params := [](interface{}){"hello1001"}
	reqmsg := jsoff.NewRequestMessage(1, "echoTyped", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello1001", res)

	// type mismatch
	params1 := [](interface{}){true}
	reqmsg1 := jsoff.NewRequestMessage(1, "echoTyped", params1)

	resmsg1, err1 := client.Call(rootCtx, reqmsg1)
	assert.Nil(err1)
	assert.True(resmsg1.IsError())
	errbody1 := resmsg1.MustError()
	assert.Equal(-32602, errbody1.Code) // params error
	assert.Contains(errbody1.Message, "got unconvertible type")

	// test params size
	params2 := [](interface{}){}
	reqmsg2 := jsoff.NewRequestMessage(2, "echoTyped", params2)

	resmsg2, err2 := client.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.True(resmsg2.IsError())
	errbody2 := resmsg2.MustError()
	assert.Equal(-32602, errbody2.Code)
	assert.Equal("no enough params size", errbody2.Message)

	// test add 2 numbers
	params3 := [](interface{}){6, 3}
	reqmsg3 := jsoff.NewRequestMessage(3, "add", params3)
	resmsg3, err3 := client.Call(rootCtx, reqmsg3)
	assert.Nil(err3)
	assert.True(resmsg3.IsResult())
	res3 := resmsg3.MustResult()
	assert.Equal(json.Number("9"), res3)

	// test add 2 numbers with typing mismatch
	params4 := [](interface{}){"6", 4}
	reqmsg4 := jsoff.NewRequestMessage(4, "add", params4)
	resmsg4, err4 := client.Call(rootCtx, reqmsg4)
	assert.Nil(err4)
	assert.True(resmsg4.IsError())
	errbody4 := resmsg4.MustError()
	assert.Equal(-32602, errbody4.Code)
	assert.Contains(errbody4.Message, "got unconvertible type")
}

func TestH2Close(t *testing.T) {
	assert := assert.New(t)

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	clientCtx, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()

	server := NewH2Handler(serverCtx, nil)
	server.Actor.On("echo", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})

	go ListenAndServe(serverCtx, "127.0.0.1:28723", server, serverTLS())
	time.Sleep(100 * time.Millisecond)

	closeCalled := make(map[int]bool)
	client := NewH2Client(urlParse("h2://127.0.0.1:28723"))
	client.SetClientTLSConfig(clientTLS())

	connectedCalled := map[int]bool{}
	client.OnConnected(func() {
		connectedCalled[0] = true
	})
	client.OnClose(func() {
		closeCalled[0] = true
	})
	// right request
	params := [](interface{}){"hello1002"}
	reqmsg := jsoff.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(clientCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())

	res := resmsg.MustResult()
	assert.Equal("hello1002", res)
	assert.True(connectedCalled[0])

	// cancel root
	cancelServer()
	time.Sleep(100 * time.Millisecond)
	assert.True(closeCalled[0])
}

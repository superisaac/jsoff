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

func TestWSServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewWSServer(rootCtx)
	server.Handler.On("echo", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsonz.ParamsError("no argument given")
		}
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28100", server)
	time.Sleep(10 * time.Millisecond)

	client := NewWSClient("ws://127.0.0.1:28100")

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

func TestTypedWSServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewWSServer(rootCtx)
	err := server.Handler.OnTyped("echoTyped", func(req *RPCRequest, v string) (string, error) {
		return v, nil
	})
	assert.Nil(err)

	err = server.Handler.OnTyped("add", func(req *RPCRequest, a, b int) (int, error) {
		return a + b, nil
	})
	assert.Nil(err)

	go ListenAndServe(rootCtx, "127.0.0.1:28101", server)
	time.Sleep(10 * time.Millisecond)

	client := NewWSClient("ws://127.0.0.1:28101")

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

func TestWSSession(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessions := make(map[int]*WSSession)
	server := NewWSServer(rootCtx)
	server.FlowControl = true
	server.Handler.On("hijackSession", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		// capture the websocket session to uplevel for
		// following usage
		s, _ := req.Data().(*WSSession)
		sessions[0] = s
		return "ok", nil
	})
	go ListenAndServe(rootCtx, "127.0.0.1:28120", server)
	time.Sleep(10 * time.Millisecond)

	receivedMsgs := make(map[int]jsonz.Message)
	client := NewWSClient("ws://127.0.0.1:28120")
	client.OnMessage(func(msg jsonz.Message) {
		// stores the latest received message, usually a server
		// pushed notification
		receivedMsgs[0] = msg
	})
	req1 := jsonz.NewRequestMessage(1, "hijackSession", nil)
	res1, err := client.Call(rootCtx, req1)
	assert.Nil(err)
	assert.Equal("ok", res1.MustResult())

	session := sessions[0]
	assert.Equal(modePassive, session.pushMode)

	// ntf1 := jsonz.NewNotifyMessage("notify0", nil)
	// // server push
	// session.Send(ntf1)
	// // under mod unlimited, the notify is sent directly to client
	// assert.Equal(0, len(session.pushBuffer))

	err = client.ActivateSession(rootCtx)
	assert.Nil(err)

	time.Sleep(10 * time.Millisecond)
	assert.Equal(modeActive, session.pushMode)

	ntf3 := jsonz.NewNotifyMessage("notify3", nil)
	// server push
	session.Send(ntf3)
	// under mod active, the session's push mode is set to passive
	// after a message sent
	assert.Equal(modePassive, session.pushMode)
	assert.Equal(0, len(session.pushBuffer))

	ntf4 := jsonz.NewNotifyMessage("notify4", nil)
	// server push
	session.Send(ntf4)
	// under mod passive, the session's pushed message will be
	// buffered instead of being sent directly to client, the
	// session will be passive until the next _session.activate
	// notification.
	assert.Equal(modePassive, session.pushMode)
	assert.Equal(1, len(session.pushBuffer))
	assert.Equal("notify4", session.pushBuffer[0].MustMethod())

	ntf5 := jsonz.NewNotifyMessage("notify5", nil)
	// server push
	session.Send(ntf5)
	// message buffered again
	assert.Equal(modePassive, session.pushMode)
	assert.Equal(2, len(session.pushBuffer))
	assert.Equal("notify4", session.pushBuffer[0].MustMethod())
	assert.Equal("notify5", session.pushBuffer[1].MustMethod())

	// activate the session again
	err = client.ActivateSession(rootCtx)
	assert.Nil(err)
	time.Sleep(10 * time.Millisecond)
	// notify4 received
	assert.Equal("notify4", receivedMsgs[0].MustMethod())

	// one buffered element is unshifted from push buffer
	assert.Equal(modePassive, session.pushMode)
	assert.Equal(1, len(session.pushBuffer))
	assert.Equal("notify5", session.pushBuffer[0].MustMethod())

	// activate the session again and again
	err = client.ActivateSession(rootCtx)
	assert.Nil(err)
	time.Sleep(10 * time.Millisecond)
	// notify5 received
	assert.Equal("notify5", receivedMsgs[0].MustMethod())

	// one buffered element is unshifted from push buffer
	assert.Equal(modePassive, session.pushMode)
	assert.Equal(0, len(session.pushBuffer))

	// activate the session again and again and again
	err = client.ActivateSession(rootCtx)
	assert.Nil(err)
	time.Sleep(10 * time.Millisecond)

	// one buffered element is unshifted from push buffer
	assert.Equal(modeActive, session.pushMode)
	assert.Equal(0, len(session.pushBuffer))
}

func TestWSClose(t *testing.T) {
	assert := assert.New(t)

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	clientCtx, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()

	server := NewWSServer(serverCtx)
	server.Handler.On("echo", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsonz.ParamsError("no argument given")
		}
	})

	go ListenAndServe(serverCtx, "127.0.0.1:28120", server)
	time.Sleep(100 * time.Millisecond)

	closeCalled := make(map[int]bool)
	client := NewWSClient("ws://127.0.0.1:28120")
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

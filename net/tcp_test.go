package jsoffnet

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsoff"
	"testing"
	"time"
)

func TestTCPServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewTCPServer(context.Background(), nil)
	server.Actor.On("echo", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})

	go server.Start(rootCtx, "127.0.0.1:21800")
	time.Sleep(10 * time.Millisecond)

	client := NewTCPClient(urlParse("tcp://127.0.0.1:21800"))
	// right request
	params := [](interface{}){"hello102"}
	reqmsg := jsoff.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello102", res)
}

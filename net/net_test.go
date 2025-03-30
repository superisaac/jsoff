package jsoffnet

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsoff"
	"net/url"
	"testing"
	"time"
)

func TestParseURL(t *testing.T) {
	assert := assert.New(t)

	url0, err0 := url.Parse("tcp://127.0.0.1:8888")
	assert.Nil(err0)
	assert.Equal("tcp", url0.Scheme)
	assert.Equal("127.0.0.1", url0.Hostname())
	assert.Equal("8888", url0.Port())

	url1, err1 := url.Parse("vsock://2:8888")
	assert.Nil(err1)
	assert.Equal("vsock", url1.Scheme)
	assert.Equal("2", url1.Hostname())
	assert.Equal("8888", url1.Port())
}

func TestTCPServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewTCPServer(context.Background(), nil)
	server.Actor.On("echo", func(params []any) (any, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})

	go server.Start(rootCtx, "127.0.0.1:21800")
	defer server.Stop()
	time.Sleep(10 * time.Millisecond)

	client := NewTCPClient(urlParse("tcp://127.0.0.1:21800"))
	// right request
	params := []any{"hello102"}
	reqmsg := jsoff.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello102", res)
}

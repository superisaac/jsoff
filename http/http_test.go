package jsonrpchttp

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsonrpc"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestServerClient(t *testing.T) {
	assert := assert.New(t)

	server := NewServer()
	server.On("echo", func(ctx context.Context, msg jsonrpc.IMessage) (interface{}, error) {
		params := msg.MustParams()
		if len(params) > 0 {
			return params[0], nil
		} else {
			return "", nil
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
	reqmsg1 := jsonrpc.NewRequestMessage(2, "echoxxx", params1)
	resmsg1, err := client.Call(context.Background(), reqmsg1)
	assert.Nil(err)
	assert.True(resmsg1.IsError())
	errbody := resmsg1.MustError()
	assert.Equal(jsonrpc.ErrMethodNotFound.Code, errbody.Code)
}

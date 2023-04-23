// interacting jsonrpc in http family specs, currently jlibhttp
// provides 3 mechanisms: the classical http/1.1, websocket and http/2
// wire protocol.
package jlibhttp

import (
	"context"
	"crypto/tls"
	"github.com/superisaac/jlib"
	"net/http"
	"net/url"
)

type ClientOptions struct {
	// client request timeout
	Timeout int `json:"timeout" yaml:"timeout"`
}

// Client is an abstract interface a client type must implement
type Client interface {
	// Returns the server URL
	ServerURL() *url.URL

	// Call a Request message and expect a Result|Error message.
	Call(ctx context.Context, reqmsg *jlib.RequestMessage) (jlib.Message, error)

	// Call a Request message and unwrap the result message into a
	// given structure, when an Error message comes it is turned
	// into a golang error object typed *jlib.ErrorBody
	UnwrapCall(ctx context.Context, reqmsg *jlib.RequestMessage, output interface{}) error

	// Send a JSONRPC message(usually a notify) to server without
	// expecting any result.
	Send(ctx context.Context, msg jlib.Message) error

	// Set the client tls config
	SetClientTLSConfig(cfg *tls.Config)

	// Set http header
	SetExtraHeader(h http.Header)

	// Is streaming
	IsStreaming() bool
}

type MessageHandler func(msg jlib.Message)
type ConnectedHandler func()
type CloseHandler func()

type Streamable interface {
	Client

	Connect(ctx context.Context) error
	OnConnected(handler ConnectedHandler) error
	OnMessage(handler MessageHandler) error
	OnClose(handler CloseHandler) error
	Wait() error
}

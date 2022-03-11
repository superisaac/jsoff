// interacting jsonrpc in http family specs, currently jsonzhttp
// provides 3 mechanisms: the classical http/1.1, websocket and a gRPC
// wire protocol.
package jsonzhttp

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/superisaac/jsonz"
	"net/http"
	"net/url"
)

// errors
// non standard Response returned by endpoints
type WrappedResponse struct {
	Response *http.Response
}

func (self WrappedResponse) Error() string {
	return fmt.Sprintf("upstream response %d", self.Response.StatusCode)
}

// Simple HTTP response to instant return
type SimpleResponse struct {
	Code int
	Body []byte
}

func (self SimpleResponse) Error() string {
	return fmt.Sprintf("%d/%s", self.Code, self.Body)
}

// Client is an abstract interface a client type must implement
type Client interface {
	// Returns the server URL
	ServerURL() *url.URL

	// Call a Request message and expect a Result|Error message.
	Call(ctx context.Context, reqmsg *jsonz.RequestMessage) (jsonz.Message, error)

	// Call a Request message and unwrap the result message into a
	// given structure, when an Error message comes it is turned
	// into a golang error object typed *jsonz.ErrorBody
	UnwrapCall(ctx context.Context, reqmsg *jsonz.RequestMessage, output interface{}) error

	// Send a JSONRPC message(usually a notify) to server without
	// expecting any result.
	Send(ctx context.Context, msg jsonz.Message) error

	// Set the client tls config
	SetClientTLSConfig(cfg *tls.Config)

	// Set http header
	SetExtraHeader(h http.Header)
}

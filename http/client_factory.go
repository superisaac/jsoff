package jsonzhttp

import (
	"github.com/pkg/errors"
	"net/url"
)

// NewClient returns an JSONRPC client whose type depends on the
// server url it wants to connect to. Currently there are 3 types of
// supported url schemes: the HTTP/1.1 client, the websocket based
// client and the self defined gRPC protocol base client, the latter
// two types are streaming clients which can accept server push
// messages.
func NewClient(serverUrl string) (Client, error) {
	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, errors.Wrap(err, "url.Parse")
	}
	switch u.Scheme {
	case "http", "https":
		// HTTP/1.1 client
		return NewH1Client(u), nil
	case "ws", "wss":
		// Websocket client
		return NewWSClient(u), nil
	case "h2", "h2c":
		// gRPC client
		return NewGRPCClient(u), nil
	default:
		return nil, errors.New("url scheme not supported")
	}
}

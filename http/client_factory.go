package jsonzhttp

import (
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonz"
	"net/http"
	"net/url"
)

type Client interface {
	Call(ctx context.Context, reqmsg *jsonz.RequestMessage, headers ...http.Header) (jsonz.Message, error)
	UnwrapCall(ctx context.Context, reqmsg *jsonz.RequestMessage, output interface{}, headers ...http.Header) error
	Send(ctx context.Context, msg jsonz.Message, headers ...http.Header) error
}

func GetClient(serverUrl string) (Client, error) {
	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, errors.Wrap(err, "url.Parse")
	}
	switch u.Scheme {
	case "http", "https":
		return NewH1Client(serverUrl), nil
	case "ws", "wss":
		return NewWSClient(serverUrl), nil
	case "h2", "h2c":
		return NewGRPCClient(serverUrl), nil
	default:
		return nil, errors.New("url scheme not supported")
	}
}

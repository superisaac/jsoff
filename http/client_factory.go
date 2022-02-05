package jsonzhttp

import (
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonz"
	"net/url"
)

type Client interface {
	Call(ctx context.Context, reqmsg *jsonz.RequestMessage) (jsonz.Message, error)
	UnwrapCall(ctx context.Context, reqmsg *jsonz.RequestMessage, output interface{}) error
	Send(ctx context.Context, msg jsonz.Message) error
}

func GetClient(serverUrl string) (Client, error) {
	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, errors.Wrap(err, "url.Parse")
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return NewHTTPClient(serverUrl), nil
	} else if u.Scheme == "ws" || u.Scheme == "wss" {
		return NewWSClient(serverUrl), nil
	}
	return nil, errors.New("bad url schema")
}

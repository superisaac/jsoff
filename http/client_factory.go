package jsonrpchttp

import (
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonrpc"
	"net/url"
)

type IClient interface {
	Call(ctx context.Context, reqmsg *jsonrpc.RequestMessage) (jsonrpc.IMessage, error)
	Send(ctx context.Context, msg jsonrpc.IMessage) error
}

func GetClient(serverUrl string) (IClient, error) {
	u, err := url.Parse(serverUrl)
	if err != nil {
		return nil, errors.Wrap(err, "url.Parse")
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return NewClient(serverUrl), nil
	} else if u.Scheme == "ws" || u.Scheme == "wss" {
		return NewWSClient(serverUrl), nil
	}
	return nil, errors.New("bad url schema")
}

package jsozhttp

import (
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsoz"
	"net/url"
)

type Client interface {
	Call(ctx context.Context, reqmsg *jsoz.RequestMessage) (jsoz.Message, error)
	Send(ctx context.Context, msg jsoz.Message) error
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

package jsonzhttp

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"io"
	"net/http"
	"net/url"
	"reflect"
)

type WSClient struct {
	StreamingClient
}

type wsTransport struct {
	ws *websocket.Conn

	client *WSClient
}

func NewWSClient(serverUrl *url.URL) *WSClient {
	if serverUrl.Scheme != "ws" && serverUrl.Scheme != "wss" {
		log.Panicf("server url %s is not websocket", serverUrl)
	}
	c := &WSClient{}
	transport := &wsTransport{client: c}
	c.InitStreaming(serverUrl, transport)
	return c
}

func (self *WSClient) String() string {
	return fmt.Sprintf("websocket client %s", self.serverUrl)
}

func (self *WSClient) ActivateSession(ctx context.Context) error {
	ntf := jsonz.NewNotifyMessage("_session.activate", nil)
	return self.Send(ctx, ntf)
}

// websocket transport methods
func (self *wsTransport) Close() {
	if self.ws != nil {
		self.ws.Close()
		self.ws = nil
	}
}

func (self wsTransport) Connected() bool {
	return self.ws != nil
}

func (self *wsTransport) mergeHeaders(headers []http.Header) http.Header {
	//merged := http.Header{}
	var merged http.Header = nil
	for _, h := range headers {
		for k, vs := range h {
			for _, v := range vs {
				if merged == nil {
					merged = make(http.Header)
				}
				merged.Add(k, v)
			}
		}
	}
	return merged
}

func (self *wsTransport) Connect(rootCtx context.Context, serverUrl *url.URL, headers ...http.Header) error {
	dailer := websocket.DefaultDialer
	dailer.TLSClientConfig = self.client.ClientTLSConfig()
	ws, _, err := dailer.Dial(serverUrl.String(), MergeHeaders(headers))
	if err != nil {
		return err
	}
	self.ws = ws
	return nil
}

func (self *wsTransport) handleWebsocketError(err error) error {
	var closeErr *websocket.CloseError
	if errors.Is(err, io.EOF) {
		log.Infof("websocket conn failed")
		return &TransportClosed{}
	} else if errors.As(err, &closeErr) {
		log.Infof("websocket close error %d %s", closeErr.Code, closeErr.Text)
		return &TransportClosed{}
	} else {
		log.Warnf("ws.ReadMessage error %s %s", reflect.TypeOf(err), err)
	}
	return err
}

func (self *wsTransport) WriteMessage(msg jsonz.Message) error {
	marshaled, err := jsonz.MessageBytes(msg)
	if err != nil {
		return err
	}

	if err := self.ws.WriteMessage(websocket.TextMessage, marshaled); err != nil {
		return self.handleWebsocketError(err)
	}
	return nil
}

func (self *wsTransport) ReadMessage() (jsonz.Message, bool, error) {
	messageType, msgBytes, err := self.ws.ReadMessage()
	if err != nil {
		return nil, false, self.handleWebsocketError(err)
	}
	if messageType != websocket.TextMessage {
		return nil, false, nil
	}

	msg, err := jsonz.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		return nil, false, err
	}
	return msg, true, nil
}

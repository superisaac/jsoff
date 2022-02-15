package jsonzhttp

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"io"
	"reflect"
)

type WSClient struct {
	StreamingClient
}

type wsTransport struct {
	ws *websocket.Conn
}

func NewWSClient(serverUrl string) *WSClient {
	c := &WSClient{}
	transport := &wsTransport{}
	c.InitStreaming(serverUrl, transport)
	return c
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

func (self *wsTransport) Connected() bool {
	return self.ws != nil
}

func (self *wsTransport) Connect(rootCtx context.Context, serverUrl string) error {
	ws, _, err := websocket.DefaultDialer.Dial(serverUrl, nil)
	if err != nil {
		return err
	}
	self.ws = ws
	return nil
}

func (self *wsTransport) WriteMessage(msg jsonz.Message) error {
	marshaled, err := jsonz.MessageBytes(msg)
	if err != nil {
		return err
	}

	if err := self.ws.WriteMessage(websocket.TextMessage, marshaled); err != nil {
		return err
	}
	return nil
}

func (self *wsTransport) ReadMessage() (jsonz.Message, bool, error) {
	messageType, msgBytes, err := self.ws.ReadMessage()
	if err != nil {
		var closeErr *websocket.CloseError
		if errors.Is(err, io.EOF) {
			log.Infof("websocket conn failed")
		} else if errors.As(err, &closeErr) {
			log.Infof("websocket close error %d %s", closeErr.Code, closeErr.Text)
		} else {
			log.Warnf("ws.ReadMessage error %s %s", reflect.TypeOf(err), err)
		}
		return nil, false, err
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

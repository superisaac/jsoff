package jlibhttp

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jlib"
	"io"
	"net"
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

func (self *wsTransport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	dailer := websocket.DefaultDialer
	dailer.TLSClientConfig = self.client.ClientTLSConfig()
	ws, _, err := dailer.Dial(serverUrl.String(), header)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			self.client.Log().Infof("websocket operror %s", opErr)
			return TransportConnectFailed
		}
		return errors.Wrap(err, "wstransport.connect")
	}
	self.ws = ws
	return nil
}

func (self *wsTransport) handleWebsocketError(err error) error {
	logger := self.client.Log()
	var closeErr *websocket.CloseError
	if errors.Is(err, io.EOF) {
		logger.Infof("websocket conn failed")
		return TransportClosed
	} else if errors.As(err, &closeErr) {
		logger.Infof("websocket close error %d %s", closeErr.Code, closeErr.Text)
		return TransportClosed
	} else {
		logger.Warnf("ws.ReadMessage error %s %s", reflect.TypeOf(err), err)
	}
	return errors.Wrap(err, "handleWebsocketError")
}

func (self *wsTransport) WriteMessage(msg jlib.Message) error {
	marshaled, err := jlib.MessageBytes(msg)
	if err != nil {
		return err
	}

	if err := self.ws.WriteMessage(websocket.TextMessage, marshaled); err != nil {
		return self.handleWebsocketError(err)
	}
	return nil
}

func (self *wsTransport) ReadMessage() (jlib.Message, bool, error) {
	messageType, msgBytes, err := self.ws.ReadMessage()
	if err != nil {
		return nil, false, self.handleWebsocketError(err)
	}
	if messageType != websocket.TextMessage {
		return nil, false, nil
	}

	msg, err := jlib.ParseBytes(msgBytes)
	if err != nil {
		self.client.Log().Warnf("bad jsonrpc message %s", msgBytes)
		return nil, false, err
	}
	return msg, true, nil
}

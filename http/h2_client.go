package jsonzhttp

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"golang.org/x/net/http2"	
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
)

type H2Client struct {
	StreamingClient
}

type h2Transport struct {
	framer *http2.Framer
	client *H2Client
}

func NewH2Client(serverUrl *url.URL) *H2Client {
	if serverUrl.Scheme != "h2" && serverUrl.Scheme != "h2c" {
		log.Panicf("server url %s is not websocket", serverUrl)
	}
	c := &H2Client{}
	transport := &h2Transport{client: c}
	c.InitStreaming(serverUrl, transport)
	return c
}

func (self *H2Client) String() string {
	return fmt.Sprintf("websocket client %s", self.serverUrl)
}

// websocket transport methods
func (self *h2Transport) Close() {
	if self.ws != nil {
		self.ws.Close()
		self.ws = nil
	}
}

func (self h2Transport) Connected() bool {
	return self.ws != nil
}

func (self *h2Transport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	dailer := websocket.DefaultDialer
	dailer.TLSClientConfig = self.client.ClientTLSConfig()
	ws, _, err := dailer.Dial(serverUrl.String(), header)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			self.client.Log().Infof("websocket operror %s", opErr)
			return TransportConnectFailed
		}
		return err
	}
	self.ws = ws
	return nil
}

func (self *h2Transport) handleWebsocketError(err error) error {
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
	return err
}

func (self *h2Transport) WriteMessage(msg jsonz.Message) error {
	marshaled, err := jsonz.MessageBytes(msg)
	if err != nil {
		return err
	}

	if err := self.ws.WriteMessage(websocket.TextMessage, marshaled); err != nil {
		return self.handleWebsocketError(err)
	}
	return nil
}

func (self *h2Transport) ReadMessage() (jsonz.Message, bool, error) {
	messageType, msgBytes, err := self.ws.ReadMessage()
	if err != nil {
		return nil, false, self.handleWebsocketError(err)
	}
	if messageType != websocket.TextMessage {
		return nil, false, nil
	}

	msg, err := jsonz.ParseBytes(msgBytes)
	if err != nil {
		self.client.Log().Warnf("bad jsonrpc message %s", msgBytes)
		return nil, false, err
	}
	return msg, true, nil
}

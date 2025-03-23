package jsoffnet

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
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

func (client *WSClient) String() string {
	return fmt.Sprintf("websocket client %s", client.serverUrl)
}

// websocket transport methods
func (t *wsTransport) Close() {
	if t.ws != nil {
		t.ws.Close()
		t.ws = nil
	}
}

func (t wsTransport) Connected() bool {
	return t.ws != nil
}

func (t *wsTransport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	dailer := websocket.DefaultDialer
	dailer.TLSClientConfig = t.client.ClientTLSConfig()
	ws, _, err := dailer.Dial(serverUrl.String(), header)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			t.client.Log().Infof("websocket operror %s", opErr)
			return TransportConnectFailed
		}
		return errors.Wrap(err, "wstransport.connect")
	}
	t.ws = ws
	return nil
}

func (t *wsTransport) handleWebsocketError(err error) error {
	logger := t.client.Log()
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

func (t *wsTransport) WriteMessage(msg jsoff.Message) error {
	marshaled, err := jsoff.MessageBytes(msg)
	if err != nil {
		return err
	}

	if err := t.ws.WriteMessage(websocket.TextMessage, marshaled); err != nil {
		return t.handleWebsocketError(err)
	}
	return nil
}

func (t *wsTransport) ReadMessage() (jsoff.Message, bool, error) {
	messageType, msgBytes, err := t.ws.ReadMessage()
	if err != nil {
		return nil, false, t.handleWebsocketError(err)
	}
	if messageType != websocket.TextMessage {
		return nil, false, nil
	}

	msg, err := jsoff.ParseBytes(msgBytes)
	if err != nil {
		t.client.Log().Warnf("bad jsonrpc message %s", msgBytes)
		return nil, false, err
	}
	return msg, true, nil
}

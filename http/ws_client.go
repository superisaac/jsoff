package jsonrpchttp

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonrpc"
	"io"
	"sync"
	"time"
)

type pendingRequest struct {
	reqmsg        *jsonrpc.RequestMessage
	resultChannel chan jsonrpc.IMessage
	expire        time.Time
}

type WSClient struct {
	serverUrl       string
	ws              *websocket.Conn
	pendingRequests sync.Map
}

func NewWSClient(serverUrl string) *WSClient {
	return &WSClient{serverUrl: serverUrl}
}

func (self *WSClient) Close() {
	if self.ws != nil {
		self.ws.Close()
		self.ws = nil
	}
}

func (self WSClient) Connected() bool {
	return self.ws != nil
}

func (self *WSClient) connect() error {
	if self.Connected() {
		// already connectd
		return nil
	}
	ws, _, err := websocket.DefaultDialer.Dial(self.serverUrl, nil)
	if err != nil {
		return errors.Wrap(err, "websocket error")
	}
	self.ws = ws
	go self.recvLoop()
	return nil
}

func (self *WSClient) recvLoop() {
	for {
		ws := self.ws
		if ws == nil {
			return
		}
		messageType, msgBytes, err := ws.ReadMessage()
		if err != nil {
			var closeErr *websocket.CloseError
			if errors.Is(err, io.EOF) {
				log.Infof("websocket conn failed")
			} else if errors.As(err, &closeErr) {
				log.Infof("websocket close error %d %s", closeErr.Code, closeErr.Text)
			} else {
				log.Warnf("ws.ReadMessage error %s", err)
			}
			self.Close()
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}

		msg, err := jsonrpc.ParseBytes(msgBytes)
		if err != nil {
			log.Warnf("bad jsonrpc message %s", msgBytes)
			return
		}

		if !msg.IsResultOrError() {
			msg.Log().Warnf("message type accepted")
			return
		}
		self.handleResult(msg)
	}
}

func (self *WSClient) handleResult(msg jsonrpc.IMessage) {
	v, loaded := self.pendingRequests.LoadAndDelete(msg.MustId())
	if !loaded {
		msg.Log().Warnf("fail to find pending request")
		return
	}

	if pending, ok := v.(*pendingRequest); ok {
		pending.resultChannel <- msg
	}
}

func (self *WSClient) expire(k interface{}, after time.Duration) {
	// ctx, cancel := context.WithCancel(rootCtx)
	// defer cancel()
	select {
	// case <- ctx.Done():
	// 	return
	case <-time.After(after):
		v, loaded := self.pendingRequests.LoadAndDelete(k)
		if loaded {
			if pending, ok := v.(*pendingRequest); ok {
				timeout := jsonrpc.ErrTimeout.ToMessage(pending.reqmsg)
				pending.resultChannel <- timeout
			}
		}
	}
}

func (self *WSClient) Call(rootCtx context.Context, reqmsg *jsonrpc.RequestMessage) (jsonrpc.IMessage, error) {
	err := self.connect()
	if err != nil {
		return nil, err
	}
	ch := make(chan jsonrpc.IMessage, 10)

	marshaled, err := jsonrpc.MessageBytes(reqmsg)
	if err != nil {
		return nil, err
	}

	if _, loaded := self.pendingRequests.Load(reqmsg.Id); loaded {
		return nil, errors.New("duplicate request Id")
	}

	p := &pendingRequest{
		reqmsg:        reqmsg,
		resultChannel: ch,
		expire:        time.Now().Add(time.Second * 10),
	}

	self.pendingRequests.Store(reqmsg.Id, p)
	err = self.ws.WriteMessage(websocket.TextMessage, marshaled)
	if err != nil {
		return nil, err
	}
	go self.expire(reqmsg.Id, time.Second*10)

	resmsg, ok := <-ch
	if !ok {
		return nil, errors.New("result channel closed")
	}
	return resmsg, nil
}

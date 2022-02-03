package jsonzhttp

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"io"
	"reflect"
	"sync"
	"time"
)

type pendingRequest struct {
	reqmsg        *jsonz.RequestMessage
	resultChannel chan jsonz.Message
	expire        time.Time
}

type WSMessageHandler func(msg jsonz.Message)

type WSClient struct {
	serverUrl       string
	ws              *websocket.Conn
	pendingRequests sync.Map
	messageHandler  WSMessageHandler
	sendChannel     chan jsonz.Message

	connectErr      error
	connectOnce     sync.Once
}

func NewWSClient(serverUrl string) *WSClient {
	return &WSClient{
		serverUrl:   serverUrl,
		sendChannel: make(chan jsonz.Message, 100),
	}
}

func (self *WSClient) Close() {
	if self.ws != nil {
		self.ws.Close()
		self.ws = nil
	}
}

func (self *WSClient) OnMessage(handler WSMessageHandler) error {
	if self.messageHandler != nil {
		return errors.New("message handler already exist!")
	}
	self.messageHandler = handler
	return nil
}

func (self WSClient) Connected() bool {
	return self.ws != nil
}

func (self *WSClient) connect() error {
	self.connectOnce.Do(func() {
		ws, _, err := websocket.DefaultDialer.Dial(self.serverUrl, nil)
		if err != nil {
			self.connectErr = err
			return
		}
		self.ws = ws
		go self.sendLoop()
		go self.recvLoop()
	})
	return self.connectErr
}

func (self *WSClient) sendLoop() {
	defer self.Close()
	for {
		select {
		case msg, ok := <-self.sendChannel:
			if !ok {
				return
			}
			if self.ws == nil {
				return
			}
			marshaled, err := jsonz.MessageBytes(msg)
			if err != nil {
				log.Warnf("marshal msg error %s", err)
				return
			}

			if err := self.ws.WriteMessage(websocket.TextMessage, marshaled); err != nil {
				log.Warnf("write warning message %s", err)
				return
			}
		}
	}
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
				log.Warnf("ws.ReadMessage error %s %s", reflect.TypeOf(err), err)
			}
			self.Close()
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}

		msg, err := jsonz.ParseBytes(msgBytes)
		if err != nil {
			log.Warnf("bad jsonrpc message %s", msgBytes)
			return
		}

		if !msg.IsResultOrError() {
			if self.messageHandler != nil {
				self.messageHandler(msg)
			} else {
				msg.Log().Debugf("no message handler found")
			}
		} else {
			self.handleResult(msg)
		}
	}
}

func (self *WSClient) handleResult(msg jsonz.Message) {
	msgId := msg.MustId()
	v, loaded := self.pendingRequests.LoadAndDelete(msgId)
	if !loaded {
		if self.messageHandler != nil {
			self.messageHandler(msg)
		}
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
				timeout := jsonz.ErrTimeout.ToMessage(pending.reqmsg)
				pending.resultChannel <- timeout
			}
		}
	}
}

func (self *WSClient) Call(rootCtx context.Context, reqmsg *jsonz.RequestMessage) (jsonz.Message, error) {
	err := self.connect()
	if err != nil {
		return nil, err
	}
	ch := make(chan jsonz.Message, 10)

	// marshaled, err := jsonz.MessageBytes(reqmsg)
	// if err != nil {
	// 	return nil, err
	// }

	if _, loaded := self.pendingRequests.Load(reqmsg.Id); loaded {
		return nil, errors.New("duplicate request Id")
	}

	p := &pendingRequest{
		reqmsg:        reqmsg,
		resultChannel: ch,
		expire:        time.Now().Add(time.Second * 10),
	}
	self.pendingRequests.Store(reqmsg.Id, p)

	err = self.Send(rootCtx, reqmsg)
	//err = self.ws.WriteMessage(websocket.TextMessage, marshaled)
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

func (self *WSClient) Send(rootCtx context.Context, msg jsonz.Message) error {
	err := self.connect()
	if err != nil {
		return err
	}

	self.sendChannel <- msg

	// marshaled, err := jsonz.MessageBytes(msg)
	// if err != nil {
	// 	return err
	// }

	// err = self.ws.WriteMessage(websocket.TextMessage, marshaled)
	// if err != nil {
	// 	return errors.Wrap(err, "websocket.WriteMessage")
	// }
	return nil
}

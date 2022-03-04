package jsonzhttp

import (
	"context"
	"crypto/tls"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"net/http"
	"sync"
	"time"
)

type pendingRequest struct {
	reqmsg        *jsonz.RequestMessage
	resultChannel chan jsonz.Message
	expire        time.Time
}

type MessageHandler func(msg jsonz.Message)
type CloseHandler func()

type TransportClosed struct {
}

func (self TransportClosed) Error() string {
	return "streaming closed"
}

// the underline transport, currently there are websocket and gRPC
// implementations
type Transport interface {
	Connect(rootCtx context.Context, serverUrl string, headers ...http.Header) error
	Close()
	Connected() bool
	ReadMessage() (msg jsonz.Message, readed bool, err error)
	WriteMessage(msg jsonz.Message) error
}

type StreamingClient struct {
	serverUrl       string
	pendingRequests sync.Map
	messageHandler  MessageHandler
	closeHandler    CloseHandler
	sendChannel     chan jsonz.Message

	transport Transport

	connectErr  error
	connectOnce sync.Once

	clientTLS *tls.Config
}

func (self *StreamingClient) SetClientTLSConfig(cfg *tls.Config) {
	self.clientTLS = cfg
}

func (self *StreamingClient) ClientTLSConfig() *tls.Config {
	return self.clientTLS
}

func (self *StreamingClient) InitStreaming(serverUrl string, transport Transport) {
	self.serverUrl = serverUrl
	self.transport = transport
	self.sendChannel = make(chan jsonz.Message, 100)
}

func (self *StreamingClient) OnMessage(handler MessageHandler) error {
	if self.messageHandler != nil {
		return errors.New("message handler already exist!")
	}
	self.messageHandler = handler
	return nil
}

func (self *StreamingClient) OnClose(handler CloseHandler) error {
	if self.closeHandler != nil {
		return errors.New("close handler already exist!")
	}
	self.closeHandler = handler
	return nil
}

func (self *StreamingClient) connect(rootCtx context.Context, headers ...http.Header) error {
	self.connectOnce.Do(func() {
		err := self.transport.Connect(rootCtx, self.serverUrl, headers...)
		if err != nil {
			self.connectErr = err
			return
		}
		go self.sendLoop()
		go self.recvLoop()
	})
	return self.connectErr
}

func (self *StreamingClient) handleError(err error) {
	var transClosed *TransportClosed
	if errors.As(err, &transClosed) {
		log.Infof("transport closed")
		self.transport.Close()
		if self.closeHandler != nil {
			self.closeHandler()
		}
	}
}

func (self *StreamingClient) Connected() bool {
	return self.transport.Connected()
}

func (self *StreamingClient) sendLoop() {
	defer self.transport.Close()
	for {
		select {
		case msg, ok := <-self.sendChannel:
			if !ok {
				return
			}
			if !self.transport.Connected() {
				return
			}

			err := self.transport.WriteMessage(msg)
			if err != nil {
				log.Warnf("write msg error %s", err)
				self.handleError(err)
				return
			}
		}
	}
}

func (self *StreamingClient) recvLoop() {
	for {
		if !self.transport.Connected() {
			return
		}
		msg, readed, err := self.transport.ReadMessage()
		if err != nil {
			self.handleError(err)
			return
		}
		if !readed {
			continue
		}

		// assert msg != nil
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

func (self *StreamingClient) handleResult(msg jsonz.Message) {
	msgId := msg.MustId()
	v, loaded := self.pendingRequests.LoadAndDelete(msgId)
	if !loaded {
		if self.messageHandler != nil {
			self.messageHandler(msg)
		}
		return
	}

	if pending, ok := v.(*pendingRequest); ok {
		if msgId != pending.reqmsg.Id {
			resmsg := msg.ReplaceId(pending.reqmsg.Id)
			pending.resultChannel <- resmsg
		} else {
			pending.resultChannel <- msg
		}
	}
}

func (self *StreamingClient) expire(k interface{}, after time.Duration) {
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

func (self *StreamingClient) UnwrapCall(rootCtx context.Context, reqmsg *jsonz.RequestMessage, output interface{}, headers ...http.Header) error {
	resmsg, err := self.Call(rootCtx, reqmsg, headers...)
	if err != nil {
		return err
	}
	if resmsg.IsResult() {
		err := jsonz.DecodeInterface(resmsg.MustResult(), output)
		if err != nil {
			return err
		}
		return nil
	} else {
		return resmsg.MustError()
	}
}

func (self *StreamingClient) Call(rootCtx context.Context, reqmsg *jsonz.RequestMessage, headers ...http.Header) (jsonz.Message, error) {
	err := self.connect(rootCtx, headers...)
	if err != nil {
		return nil, err
	}
	ch := make(chan jsonz.Message, 10)

	sendmsg := reqmsg
	if _, loaded := self.pendingRequests.Load(reqmsg.Id); loaded {
		sendmsg = reqmsg.Clone(jsonz.NewUuid())
	}

	p := &pendingRequest{
		reqmsg:        reqmsg,
		resultChannel: ch,
		expire:        time.Now().Add(time.Second * 10),
	}
	self.pendingRequests.Store(sendmsg.Id, p)

	err = self.Send(rootCtx, sendmsg, headers...)
	if err != nil {
		return nil, err
	}
	go self.expire(sendmsg.Id, time.Second*10)

	resmsg, ok := <-ch
	if !ok {
		return nil, errors.New("result channel closed")
	}
	return resmsg, nil
}

func (self *StreamingClient) Send(rootCtx context.Context, msg jsonz.Message, headers ...http.Header) error {
	err := self.connect(rootCtx, headers...)
	if err != nil {
		return err
	}
	self.sendChannel <- msg
	return nil
}

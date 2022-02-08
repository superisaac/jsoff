package jsonzhttp

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"net/http"
)

const (
	modeUnlimited = iota
	modeActive
	modePassive
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  10240,
	WriteBufferSize: 10240,
}

type WSServer struct {
	Handler *Handler
	// options
	SpawnGoroutine bool
	FlowControl    bool
}

type WSSession struct {
	server      *WSServer
	ws          *websocket.Conn
	httpRequest *http.Request
	rootCtx     context.Context
	done        chan error
	sendChannel chan jsonz.Message
	pushMode    int
	pushBuffer  []jsonz.Message
}

func NewWSServer() *WSServer {
	return NewWSServerFromHandler(nil)
}

func NewWSServerFromHandler(handler *Handler) *WSServer {
	if handler == nil {
		handler = NewHandler()
	}
	return &WSServer{
		Handler: handler,
	}
}

func (self *WSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warnf("ws upgrade failed %s", err)
		w.WriteHeader(400)
		w.Write([]byte("ws upgrade failed"))
		return
	}
	defer ws.Close()
	defer func() {
		self.Handler.HandleClose(r)
	}()

	pushMode := modeUnlimited
	if self.FlowControl {
		pushMode = modePassive
	}

	session := &WSSession{
		server:      self,
		rootCtx:     r.Context(),
		httpRequest: r,
		ws:          ws,
		done:        make(chan error, 10),
		sendChannel: make(chan jsonz.Message, 100),
		pushMode:    pushMode,
		pushBuffer:  make([]jsonz.Message, 0),
	}
	session.wait()
	session.server = nil
}

// websocket session
func (self *WSSession) wait() {
	ctx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	go self.sendLoop()
	go self.recvLoop()

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-self.done:
			if ok && err != nil {
				log.Warnf("websocket error %s", err)
			}
			return
		}
	}
}

func (self *WSSession) recvLoop() {
	for {
		messageType, msgBytes, err := self.ws.ReadMessage()
		if err != nil {
			self.done <- errors.Wrap(err, "ws.ReadMessage()")
			return
		}
		if messageType != websocket.TextMessage {
			log.Infof("message type %d is not text, wait for next", messageType)
			continue
		}

		if self.server.SpawnGoroutine {
			go self.msgBytesReceived(msgBytes)
		} else {
			self.msgBytesReceived(msgBytes)
		}
	}
}

func (self *WSSession) activateSession() {
	if !self.server.FlowControl {
		return
	}
	self.pushMode = modeActive
	if len(self.pushBuffer) > 0 {
		msg := self.pushBuffer[0]
		self.pushBuffer = self.pushBuffer[1:]
		self.sendChannel <- msg
		self.pushMode = modePassive
	}
}

func (self *WSSession) msgBytesReceived(msgBytes []byte) {
	msg, err := jsonz.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		self.done <- errors.New("bad jsonrpc message")
		return
	}

	if msg.IsNotify() && msg.MustMethod() == "_session.activate" {
		self.activateSession()
		return
	}

	req := NewRPCRequest(
		self.rootCtx,
		msg,
		self.httpRequest,
		self)

	resmsg, err := self.server.Handler.HandleRequest(req)
	if err != nil {
		self.done <- errors.Wrap(err, "handler.handlerRequest")
		return
	}
	if resmsg != nil {
		if resmsg.IsResultOrError() {
			self.sendChannel <- resmsg
		} else {
			self.Send(resmsg)
		}
	}
}

func (self *WSSession) Send(msg jsonz.Message) {
	switch self.pushMode {
	case modeUnlimited:
		self.sendChannel <- msg
	case modeActive:
		self.sendChannel <- msg
		self.pushMode = modePassive
	case modePassive:
		// TODO: synchronization around pushBuffer
		self.pushBuffer = append(self.pushBuffer, msg)
		if len(self.pushBuffer) > 100 {
			log.Warnf("too many messages strucked in push buffer, len(pushBuffer) = %d", len(self.pushBuffer))
		}
	}
}

func (self *WSSession) sendLoop() {
	ctx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
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

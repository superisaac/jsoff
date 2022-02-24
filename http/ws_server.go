package jsonzhttp

import (
	//"fmt"
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

type WSHandler struct {
	Actor     *Actor
	serverCtx context.Context
	// options
	SpawnGoroutine bool
}

type WSSession struct {
	server      *WSHandler
	ws          *websocket.Conn
	httpRequest *http.Request
	rootCtx     context.Context
	done        chan error
	sendChannel chan jsonz.Message
}

func NewWSHandler(serverCtx context.Context, actor *Actor) *WSHandler {
	if actor == nil {
		actor = NewActor()
	}
	return &WSHandler{
		serverCtx: serverCtx,
		Actor:     actor,
	}
}

func (self *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warnf("ws upgrade failed %s", err)
		w.WriteHeader(400)
		w.Write([]byte("ws upgrade failed"))
		return
	}
	defer ws.Close()
	defer func() {
		self.Actor.HandleClose(r)
	}()

	session := &WSSession{
		server:      self,
		rootCtx:     r.Context(),
		httpRequest: r,
		ws:          ws,
		done:        make(chan error, 10),
		sendChannel: make(chan jsonz.Message, 100),
	}
	session.wait()
	session.server = nil
}

// websocket session
func (self *WSSession) wait() {
	connCtx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	serverCtx, cancelServer := context.WithCancel(self.server.serverCtx)
	defer cancelServer()

	go self.sendLoop()
	go self.recvLoop()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-serverCtx.Done():
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

func (self *WSSession) msgBytesReceived(msgBytes []byte) {
	msg, err := jsonz.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		self.done <- errors.New("bad jsonrpc message")
		return
	}

	req := NewRPCRequest(
		self.rootCtx,
		msg,
		TransportWebsocket,
		self.httpRequest,
		self)

	resmsg, err := self.server.Actor.Feed(req)
	if err != nil {
		self.done <- errors.Wrap(err, "actor.Feed")
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
	self.sendChannel <- msg
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

package jsonrpchttp

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonrpc"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  10240,
	WriteBufferSize: 10240,
}

type WSServer struct {
	SpawnGoroutine bool
	dispatcher     *Dispatcher
}

func NewWSServer(dispatcher *Dispatcher) *WSServer {
	if dispatcher == nil {
		dispatcher = NewDispatcher()
	}
	return &WSServer{
		dispatcher: dispatcher,
	}
}

func (self *WSServer) ServerHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warnf("ws upgrade failed %s", err)
		w.WriteHeader(400)
		w.Write([]byte("ws upgrade failed"))
		return
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	done := make(chan error, 10)
	go self.recvLoop(r.Context(), ws, done)

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-done:
			if ok && err != nil {
				log.Warnf("websocket error %s", err)
			}
			return
		}
	}
}

func (self *WSServer) recvLoop(rootCtx context.Context, ws *websocket.Conn, done chan error) {
	for {
		messageType, msgBytes, err := ws.ReadMessage()
		if err != nil {
			done <- errors.Wrap(err, "ws.ReadMessage()")
			return
		}
		if messageType != websocket.TextMessage {
			log.Infof("message type %d is not text, wait for next", messageType)
			continue
		}

		if self.SpawnGoroutine {
			go self.handleWSBytes(rootCtx, msgBytes, ws, done)
		} else {
			self.handleWSBytes(rootCtx, msgBytes, ws, done)
		}
	}
}

func (self *WSServer) handleWSBytes(rootCtx context.Context, msgBytes []byte, ws *websocket.Conn, done chan error) {
	msg, err := jsonrpc.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		done <- errors.New("bad jsonrpc message")
		return
	}

	resmsg, err := self.dispatcher.handleMessage(rootCtx, msg)
	if err != nil {
		done <- errors.Wrap(err, "dispatcher.handleMessage")
		return
	}
	if msg.IsRequest() {
		if resmsg == nil {
			msg.Log().Panicf("result message should not be nil")
			done <- errors.New("result msg should be nil")
			return
		}

		resMsgBytes, err := jsonrpc.MessageBytes(resmsg)
		if err != nil {
			done <- errors.Wrap(err, "MessageBytes")
			return
		}
		err = ws.WriteMessage(websocket.TextMessage, resMsgBytes)
		if err != nil {
			done <- errors.Wrap(err, "webocket.WriteMessage")
		}
	}
}

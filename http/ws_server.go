package jsonzhttp

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  10240,
	WriteBufferSize: 10240,
}

type WSServer struct {
	SpawnGoroutine bool
	Router         *Router
}

func NewWSServer(router *Router) *WSServer {
	if router == nil {
		router = NewRouter()
	}
	return &WSServer{
		Router: router,
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	done := make(chan error, 10)
	go self.recvLoop(r.Context(), ws, r, done)

	defer func() {
		if self.Router.closeHandler != nil {
			self.Router.closeHandler(r)
		}
	}()

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

func (self *WSServer) recvLoop(rootCtx context.Context, ws *websocket.Conn, r *http.Request, done chan error) {
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
			go self.handleWSBytes(rootCtx, msgBytes, ws, r, done)
		} else {
			self.handleWSBytes(rootCtx, msgBytes, ws, r, done)
		}
	}
}

func (self *WSServer) handleWSBytes(rootCtx context.Context, msgBytes []byte, ws *websocket.Conn, r *http.Request, done chan error) {
	msg, err := jsonz.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		done <- errors.New("bad jsonrpc message")
		return
	}

	req := &RPCRequest{context: rootCtx, msg: msg, r: r, data: ws}
	resmsg, err := self.Router.handleRequest(req)
	if err != nil {
		done <- errors.Wrap(err, "router.handleMessage")
		return
	}
	if resmsg != nil {
		err := self.SendMessage(ws, resmsg)
		if err != nil {
			done <- err
			return
		}
	}
}

func (self WSServer) SendMessage(ws *websocket.Conn, msg jsonz.Message) error {
	bytes, err := jsonz.MessageBytes(msg)
	if err != nil {
		return err
	}
	err = ws.WriteMessage(websocket.TextMessage, bytes)
	if err != nil {
		return errors.Wrap(err, "websocket.WriteMessage")
	}
	return nil
}

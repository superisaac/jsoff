package jsoffnet

import (
	//"fmt"
	"context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
	"net/http"
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
	sendChannel chan jsoff.Message
	sessionId   string
}

func NewWSHandler(serverCtx context.Context, actor *Actor) *WSHandler {
	if actor == nil {
		actor = NewActor()
	}
	return &WSHandler{
		serverCtx:      serverCtx,
		Actor:          actor,
		SpawnGoroutine: true,
	}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warnf("ws upgrade failed %s", err)
		w.WriteHeader(400)
		w.Write([]byte("ws upgrade failed"))
		return
	}
	defer ws.Close()

	session := &WSSession{
		server:      h,
		rootCtx:     r.Context(),
		httpRequest: r,
		ws:          ws,
		done:        make(chan error, 10),
		sendChannel: make(chan jsoff.Message, 100),
		sessionId:   jsoff.NewUuid(),
	}
	defer func() {
		h.Actor.HandleClose(session)
	}()
	session.wait()
	session.server = nil
}

// websocket session
func (session *WSSession) wait() {
	connCtx, cancel := context.WithCancel(session.rootCtx)
	defer cancel()

	serverCtx, cancelServer := context.WithCancel(session.server.serverCtx)
	defer cancelServer()

	go session.sendLoop()
	go session.recvLoop()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-serverCtx.Done():
			return
		case err, ok := <-session.done:
			if ok && err != nil {
				log.Warnf("websocket error %s", err)
			}
			return
		}
	}
}

func (session *WSSession) recvLoop() {
	for {
		messageType, msgBytes, err := session.ws.ReadMessage()
		if err != nil {
			session.done <- errors.Wrap(err, "ws.ReadMessage()")
			return
		}
		if messageType != websocket.TextMessage {
			log.Infof("message type %d is not text, wait for next", messageType)
			continue
		}

		if session.server.SpawnGoroutine {
			go session.msgBytesReceived(msgBytes)
		} else {
			session.msgBytesReceived(msgBytes)
		}
	}
}

func (session *WSSession) msgBytesReceived(msgBytes []byte) {
	msg, err := jsoff.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		session.done <- errors.New("bad jsonrpc message")
		return
	}

	req := NewRPCRequest(
		session.rootCtx,
		msg,
		TransportWebsocket).WithHTTPRequest(session.httpRequest).WithSession(session)

	resmsg, err := session.server.Actor.Feed(req)
	if err != nil {
		session.done <- errors.Wrap(err, "actor.Feed")
		return
	}
	if resmsg != nil {
		if resmsg.IsResultOrError() {
			session.sendChannel <- resmsg
		} else {
			session.Send(resmsg)
		}
	}
}

func (session *WSSession) Send(msg jsoff.Message) {
	session.sendChannel <- msg
}

func (session WSSession) Context() context.Context {
	return session.rootCtx
}

func (session WSSession) SessionID() string {
	return session.sessionId
}

func (session *WSSession) sendLoop() {
	ctx, cancel := context.WithCancel(session.rootCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-session.sendChannel:
			if !ok {
				return
			}
			if session.ws == nil {
				return
			}
			marshaled, err := jsoff.MessageBytes(msg)
			if err != nil {
				log.Warnf("marshal msg error %s", err)
				return
			}

			if err := session.ws.WriteMessage(websocket.TextMessage, marshaled); err != nil {
				log.Warnf("write warning message %s", err)
				return
			}
		}
	}
}

package jsonzhttp

import (
	"context"
	"net/http"
)

// shared handler serve http/websocket/grpc server over the same port
// using http protocol detection.
//
// NOTE: smart handler must work over TLS to serve h2
type SmartHandler struct {
	h1Handler *H1Handler
	wsHandler *WSHandler
	h2Handler *H2Handler
	Actor     *Actor
}

func NewSmartHandler(serverCtx context.Context, actor *Actor) *SmartHandler {
	if actor == nil {
		actor = NewActor()
	}
	return &SmartHandler{
		Actor:     actor,
		h1Handler: NewH1Handler(actor),
		wsHandler: NewWSHandler(serverCtx, actor),
		h2Handler: NewH2Handler(serverCtx, actor),
	}
}

func (self *SmartHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoAtLeast(2, 0) {
		// http2 and content type is grpc
		self.h2Handler.ServeHTTP(w, r)
	} else if r.Header.Get("Sec-Websocket-Key") != "" {
		// maybe websocket handler
		self.wsHandler.ServeHTTP(w, r)
	} else {
		// fail over to http1 handler
		self.h1Handler.ServeHTTP(w, r)
	}
}

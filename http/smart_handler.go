package jsonzhttp

import (
	"context"
	"net/http"
)

// shared handler serve http1/http2/websocket server over the same port
// using http protocol detection.
//
// NOTE: smart handler must work over TLS to serve h2
type SmartHandler struct {
	h1Handler http.Handler
	wsHandler http.Handler
	h2Handler http.Handler
	Actor     *Actor
}

func NewSmartHandler(serverCtx context.Context, actor *Actor, Insecure bool) *SmartHandler {
	if actor == nil {
		actor = NewActor()
	}

	sh := &SmartHandler{
		Actor:     actor,
		h1Handler: NewH1Handler(actor),
		wsHandler: NewWSHandler(serverCtx, actor),
	}

	if Insecure {
		sh.h2Handler = NewH2Handler(serverCtx, actor).H2CHandler()
	} else {
		sh.h2Handler = NewH2Handler(serverCtx, actor)
	}
	return sh
}

func (self *SmartHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoAtLeast(2, 0) {
		// http2 check by proto
		self.h2Handler.ServeHTTP(w, r)
	} else if r.Header.Get("Upgrade") == "h2c" {
		// h2c upgrade
		self.h2Handler.ServeHTTP(w, r)
	} else if r.Header.Get("Upgrade") == "websocket" {
		// maybe websocket handler
		self.wsHandler.ServeHTTP(w, r)
	} else {
		// fail over to http1 handler
		self.h1Handler.ServeHTTP(w, r)
	}
}

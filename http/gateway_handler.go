package jlibhttp

import (
	"context"
	"net/http"
)

// shared handler serve http1/http2/websocket server over the same port
// using http protocol detection.
//
// NOTE: gateway handler must work over TLS to serve h2
type GatewayHandler struct {
	h1Handler http.Handler
	wsHandler http.Handler
	h2Handler http.Handler
	Actor     *Actor
	insecure  bool
}

func NewGatewayHandler(serverCtx context.Context, actor *Actor, insecure bool) *GatewayHandler {
	if actor == nil {
		actor = NewActor()
	}

	sh := &GatewayHandler{
		Actor:     actor,
		h1Handler: NewH1Handler(actor),
		wsHandler: NewWSHandler(serverCtx, actor),
		insecure:  insecure,
	}

	if insecure {
		sh.h2Handler = NewH2Handler(serverCtx, actor).H2CHandler()
	} else {
		sh.h2Handler = NewH2Handler(serverCtx, actor)
	}
	return sh
}

func (self *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoAtLeast(2, 0) {
		// http2 check by proto
		self.h2Handler.ServeHTTP(w, r)
		return
	}

	upgradeHeader := r.Header.Get("Upgrade")
	if upgradeHeader == "websocket" {
		// maybe websocket handler
		self.wsHandler.ServeHTTP(w, r)
		return
	}

	if upgradeHeader == "h2c" {
		// maybe http2c handler
		self.h2Handler.ServeHTTP(w, r)
		return
	}

	// fail over to http1 handler
	self.h1Handler.ServeHTTP(w, r)
	return

}

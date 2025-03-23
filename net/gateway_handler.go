package jsoffnet

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
		h1Handler: NewHttp1Handler(actor),
		wsHandler: NewWSHandler(serverCtx, actor),
		insecure:  insecure,
	}

	if insecure {
		sh.h2Handler = NewHttp2Handler(serverCtx, actor).Http2CHandler()
	} else {
		sh.h2Handler = NewHttp2Handler(serverCtx, actor)
	}
	return sh
}

func (handler *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoAtLeast(2, 0) {
		// http2 check by proto
		handler.h2Handler.ServeHTTP(w, r)
		return
	}

	upgradeHeader := r.Header.Get("Upgrade")
	if upgradeHeader == "websocket" {
		// maybe websocket handler
		handler.wsHandler.ServeHTTP(w, r)
		return
	}

	if upgradeHeader == "h2c" {
		// maybe http2c handler
		handler.h2Handler.ServeHTTP(w, r)
		return
	}

	// fail over to http1 handler
	handler.h1Handler.ServeHTTP(w, r)
}

package jsonzhttp

import (
	"context"
	grpc "google.golang.org/grpc"
	"net/http"
	"strings"
)

// shared handler serve http/websocket/grpc server over the same port
// using http protocol detection.
// 
// NOTE: smart handler must work over TLS to serve gRPC
type SmartHandler struct {
	h1Handler   *H1Server
	wsHandler   *WSServer
	grpcHandler *grpc.Server
}

func NewSmartHandler(serverCtx context.Context, actor *Actor) *SmartHandler {
	if actor == nil {
		actor = NewActor()
	}
	grpcServer := NewGRPCServer(serverCtx, actor)
	return &SmartHandler{
		h1Handler:   NewH1Server(actor),
		wsHandler:   NewWSServer(serverCtx, actor),
		grpcHandler: grpcServer.ServerHandler(),
	}
}

func (self *SmartHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoMajor == 2 && strings.HasPrefix(
		r.Header.Get("Content-Type"), "application/grpc") {
		// http2 and content type is grpc
		self.grpcHandler.ServeHTTP(w, r)
	} else if r.Header.Get("Sec-Websocket-Key") != "" {
		// maybe websocket handler
		self.wsHandler.ServeHTTP(w, r)
	} else {
		// fail over to http1 handler
		self.h1Handler.ServeHTTP(w, r)
	}
}

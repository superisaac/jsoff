package jsonrpchttp

import (
	"bytes"
	"context"
	//"fmt"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonrpc"
	"net/http"
)

type Server struct {
	dispatcher *Dispatcher
}

func NewServer(dispatcher *Dispatcher) *Server {
	return &Server{
		dispatcher: dispatcher,
	}
}

func (self *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// only support POST
	if r.Method != "POST" {
		jsonrpc.ErrorResponse(w, r, errors.New("method not allowed"), 405, "Method not allowed")
		return
	}

	// parsing http body
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		jsonrpc.ErrorResponse(w, r, err, 400, "Bad request")
		return
	}

	msg, err := jsonrpc.ParseBytes(buffer.Bytes())
	if err != nil {
		jsonrpc.ErrorResponse(w, r, err, 400, "Bad jsonrpc request")
		return
	}

	if !msg.IsRequestOrNotify() {
		jsonrpc.ErrorResponse(w, r, err, 400, "Bad request, must be request or notify")
		return
	}

	var resmsg jsonrpc.IMessage
	var reqmsg *jsonrpc.RequestMessage

	if msg.IsRequest() {
		reqmsg, _ = msg.(*jsonrpc.RequestMessage)
	}

	if handler, found := self.dispatcher.getHandler(msg.MustMethod()); found {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		req := &RPCRequest{context: ctx, msg: msg}
		params := msg.MustParams()
		res, err := handler(req, params)
		if err != nil {
			if msg.IsRequest() {
				if rpcErr, ok := err.(*jsonrpc.RPCError); ok {
					resmsg = rpcErr.ToMessage(reqmsg)
				} else {
					resmsg = jsonrpc.ErrInternalError.ToMessage(reqmsg)
				}
			} else {
				msg.Log().Warnf("error %s", err)
			}
		} else if resmsg1, ok := res.(jsonrpc.IMessage); ok {
			// normal response
			resmsg = resmsg1
		} else {
			resmsg = jsonrpc.NewResultMessage(reqmsg, res)
		}
	} else {
		if msg.IsRequest() {
			resmsg = jsonrpc.ErrMethodNotFound.ToMessage(reqmsg)
		} else {
			resmsg = nil
		}

	}

	if msg.IsRequest() {
		if resmsg == nil {
			msg.Log().Panicf("resmsg is nil")
		}
		traceId := resmsg.TraceId()
		resmsg.SetTraceId("")

		data, err1 := jsonrpc.MessageBytes(resmsg)
		if err1 != nil {
			resmsg.Log().Warnf("error marshaling msg %s", err1)
			errmsg := jsonrpc.ErrInternalError.ToMessage(reqmsg)
			data, _ = jsonrpc.MessageBytes(errmsg)
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")

		if traceId != "" {
			w.Header().Set("X-Trace-Id", traceId)
		}
		w.Write(data)
	} else {
		w.WriteHeader(200)
		w.Write([]byte(""))
	}
} // Server.ServeHTTP

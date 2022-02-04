package jsonzhttp

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonz"
	"io"
	"net/http"
)

type Server struct {
	Router *Router
}

func NewServer(router *Router) *Server {
	if router == nil {
		router = NewRouter()
	}
	return &Server{
		Router: router,
	}
}

func (self *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// only support POST
	if r.Method != "POST" {
		jsonz.ErrorResponse(w, r, errors.New("method not allowed"), 405, "Method not allowed")
		return
	}

	// parsing http body
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		jsonz.ErrorResponse(w, r, err, 400, "Bad request")
		return
	}

	msg, err := jsonz.ParseBytes(buffer.Bytes())
	if err != nil {
		jsonz.ErrorResponse(w, r, err, 400, "Bad jsonrpc request")
		return
	}

	req := &RPCRequest{context: r.Context(), msg: msg, r: r}
	resmsg, err := self.Router.handleRequest(req)
	if err != nil {
		var simpleResp *SimpleResponse
		var upResp *WrappedResponse
		if errors.As(err, &simpleResp) {
			w.WriteHeader(simpleResp.Code)
			w.Write(simpleResp.Body)
			return
		} else if errors.As(err, &upResp) {
			origResp := upResp.Response
			for hn, hvs := range origResp.Header {
				for _, hv := range hvs {
					w.Header().Add(hn, hv)
				}
			}
			w.WriteHeader(origResp.StatusCode)
			io.Copy(w, origResp.Body)
			return
		}
		msg.Log().Warnf("err.handleMessage %s", err)
		w.WriteHeader(500)
		w.Write([]byte("internal server error"))
		return
	}
	//if msg.IsRequest() {
	if resmsg != nil {
		traceId := resmsg.TraceId()
		resmsg.SetTraceId("")

		data, err1 := jsonz.MessageBytes(resmsg)
		if err1 != nil {
			resmsg.Log().Warnf("error marshaling msg %s", err1)
			errmsg := jsonz.ErrInternalError.ToMessageFromId(msg.MustId(), msg.TraceId())
			data, _ = jsonz.MessageBytes(errmsg)
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

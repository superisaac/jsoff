package jlibhttp

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/superisaac/jlib"
	//"io"
	"net/http"
)

type H1Handler struct {
	Actor *Actor
}

func NewH1Handler(actor *Actor) *H1Handler {
	if actor == nil {
		actor = NewActor()
	}
	return &H1Handler{
		Actor: actor,
	}
}

func (self *H1Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// only support POST
	if r.Method != "POST" {
		jlib.ErrorResponse(w, r, errors.New("method not allowed"), http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// parsing http body
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		jlib.ErrorResponse(w, r, err, 400, "Bad request")
		return
	}

	msg, err := jlib.ParseBytes(buffer.Bytes())
	if err != nil {
		jlib.ErrorResponse(w, r, err, 400, "Bad jsonrpc request")
		return
	}

	req := NewRPCRequest(r.Context(), msg, TransportHTTP, r)
	resmsg, err := self.Actor.Feed(req)
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

			_, err := w.Write(upResp.Buffer.Bytes())
			//_, err := io.Copy(w, origResp.Body)
			if err != nil {
				msg.Log().Errorf("Write buffer error %#v", err)
			}
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

		data, err1 := jlib.MessageBytes(resmsg)
		if err1 != nil {
			resmsg.Log().Warnf("error marshaling msg %s", err1)
			errmsg := jlib.ErrInternalError.ToMessageFromId(msg.MustId(), msg.TraceId())
			data, _ = jlib.MessageBytes(errmsg)
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

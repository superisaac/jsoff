package jsoffnet

import (
	"bytes"
	"strings"

	"github.com/pkg/errors"
	"github.com/superisaac/jsoff"

	//"io"
	"net/http"
)

type Http1Handler struct {
	Actor *Actor
}

func NewHttp1Handler(actor *Actor) *Http1Handler {
	if actor == nil {
		actor = NewActor()
	}
	return &Http1Handler{
		Actor: actor,
	}
}

func (self Http1Handler) WriteMessage(w http.ResponseWriter, msg jsoff.Message, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(jsoff.MustMessageBytes(msg))
}

func (self *Http1Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// only support POST
	if r.Method != "POST" {
		jsoff.ErrorResponse(w, r, errors.New("method not allowed"), http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// parsing http body
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		//jsoff.ErrorResponse(w, r, err, 400, "Bad request")
		errMsg := jsoff.NewErrorMessage(nil, jsoff.ErrInvalidRequest)
		self.WriteMessage(w, errMsg, 400)
		return
	}

	msg, err := jsoff.ParseBytes(buffer.Bytes())
	if err != nil {
		//	jsoff.ErrorResponse(w, r, err, 400, "Bad jsonrpc request")
		errMsg := jsoff.NewErrorMessage(nil, jsoff.ErrParseMessage)
		self.WriteMessage(w, errMsg, 400)
		return
	}

	if discoverReqmsg, ok := msg.(*jsoff.RequestMessage); ok && discoverReqmsg.Method == "rpc.discover" {
		discoverResult := jsoff.NewResultMessage(discoverReqmsg, map[string]any{
			"methods": self.Actor.PublicSchemas(),
		})
		self.WriteMessage(w, discoverResult, 200)
		return
	}

	req := NewRPCRequest(r.Context(), msg, TransportHTTP).WithHTTPRequest(r)
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

			_, err := w.Write(upResp.Body)
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

	if resmsg != nil {
		traceId := resmsg.TraceId()
		resmsg.SetTraceId("")

		data, err1 := jsoff.MessageBytes(resmsg)
		if err1 != nil {
			resmsg.Log().Warnf("error marshaling msg %s", err1)
			errmsg := jsoff.ErrInternalError.ToMessageFromId(msg.MustId(), msg.TraceId())
			data, _ = jsoff.MessageBytes(errmsg)
		}

		w.Header().Set("Content-Type", "application/json")

		if responseMsg, ok := resmsg.(jsoff.ResponseMessage); ok && responseMsg.HasResponseHeader() {
			for header, values := range responseMsg.ResponseHeader() {
				// only transfer X- prefixed headers
				if strings.HasPrefix(strings.ToUpper(header), "X-") {
					for _, value := range values {
						w.Header().Add(header, value)
					}
				}
			}
		}

		w.WriteHeader(200)
		if traceId != "" {
			w.Header().Set("X-Trace-Id", traceId)
		}
		w.Write(data)
	} else {
		okMsg := jsoff.NewResultMessage(nil, "ok")
		self.WriteMessage(w, okMsg, 200)
	}
} // Server.ServeHTTP

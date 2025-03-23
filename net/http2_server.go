package jsoffnet

import (
	"context"
	"encoding/json"

	"github.com/superisaac/jsoff"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"io"
	"net/http"
	"sync"
)

type Http2Handler struct {
	Actor     *Actor
	serverCtx context.Context
	// options
	SpawnGoroutine bool
	UseHttp2C      bool

	fallbackHandler *Http1Handler
	fallbackOnce    sync.Once
}

type Http2Session struct {
	server      *Http2Handler
	decoder     *json.Decoder
	writer      io.Writer
	flusher     http.Flusher
	httpRequest *http.Request
	rootCtx     context.Context
	done        chan error
	sendChannel chan jsoff.Message
	sessionId   string
}

func NewHttp2Handler(serverCtx context.Context, actor *Actor) *Http2Handler {
	if actor == nil {
		actor = NewActor()
	}
	return &Http2Handler{
		serverCtx:      serverCtx,
		Actor:          actor,
		SpawnGoroutine: true,
	}
}

func (h *Http2Handler) Http2CHandler() http.Handler {
	h.UseHttp2C = true
	h2server := &http2.Server{}
	return h2c.NewHandler(h, h2server)
}

func (h *Http2Handler) FallbackHandler() *Http1Handler {
	h.fallbackOnce.Do(func() {
		h.fallbackHandler = NewHttp1Handler(h.Actor)
	})
	return h.fallbackHandler
}

func (h *Http2Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.UseHttp2C && !r.ProtoAtLeast(2, 0) {
		//return fmt.Errorf("HTTP2 not supported")
		//w.WriteHeader(400)
		//w.Write([]byte("http2 not supported"))
		h.FallbackHandler().ServeHTTP(w, r)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(400)
		w.Write([]byte("http2 not supported"))
		return
	}

	w.WriteHeader(http.StatusOK)
	//w.Write([]byte("{\"method\":\"hello\",\"params\":[]}\n"))
	flusher.Flush()

	decoder := json.NewDecoder(r.Body)
	session := &Http2Session{
		server:      h,
		rootCtx:     r.Context(),
		httpRequest: r,
		writer:      w,
		flusher:     flusher,
		decoder:     decoder,
		done:        make(chan error, 10),
		sendChannel: make(chan jsoff.Message, 100),
		sessionId:   jsoff.NewUuid(),
	}
	defer func() {
		r.Body.Close()
		h.Actor.HandleClose(session)
	}()
	session.wait()
}

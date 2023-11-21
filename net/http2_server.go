package jsoffnet

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

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

func (self *Http2Handler) Http2CHandler() http.Handler {
	self.UseHttp2C = true
	h2server := &http2.Server{}
	return h2c.NewHandler(self, h2server)
}

func (self *Http2Handler) FallbackHandler() *Http1Handler {
	self.fallbackOnce.Do(func() {
		self.fallbackHandler = NewHttp1Handler(self.Actor)
	})
	return self.fallbackHandler
}

func (self *Http2Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !self.UseHttp2C && !r.ProtoAtLeast(2, 0) {
		//return fmt.Errorf("HTTP2 not supported")
		//w.WriteHeader(400)
		//w.Write([]byte("http2 not supported"))
		self.FallbackHandler().ServeHTTP(w, r)
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
		server:      self,
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
		self.Actor.HandleClose(session)
	}()
	session.wait()
}

// http2 session
func (self *Http2Session) wait() {
	connCtx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	serverCtx, cancelServer := context.WithCancel(self.server.serverCtx)
	defer cancelServer()

	go self.sendLoop()
	go self.recvLoop()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-serverCtx.Done():
			return
		case err, ok := <-self.done:
			if ok && err != nil {
				log.Warnf("websocket error %s", err)
			}
			return
		}
	}
}

func (self *Http2Session) recvLoop() {
	for {
		msg, err := jsoff.DecodeMessage(self.decoder)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				self.done <- err
				return
			}
		}
		if self.server.SpawnGoroutine {
			go self.msgReceived(msg)
		} else {
			self.msgReceived(msg)
		}
	}
	// end of scanning
	self.done <- nil
	return
}

func (self *Http2Session) msgReceived(msg jsoff.Message) {
	req := NewRPCRequest(
		self.rootCtx,
		msg,
		TransportHTTP2).WithHTTPRequest(self.httpRequest).WithSession(self)

	resmsg, err := self.server.Actor.Feed(req)
	if err != nil {
		self.done <- errors.Wrap(err, "actor.Feed")
		return
	}
	if resmsg != nil {
		if resmsg.IsResultOrError() {
			self.sendChannel <- resmsg
		} else {
			self.Send(resmsg)
		}
	}
}

func (self *Http2Session) Send(msg jsoff.Message) {
	self.sendChannel <- msg
}

func (self Http2Session) SessionID() string {
	return self.sessionId
}

func (self *Http2Session) sendLoop() {
	ctx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-self.sendChannel:
			if !ok {
				return
			}
			if self.decoder == nil {
				return
			}
			marshaled, err := jsoff.MessageBytes(msg)
			if err != nil {
				log.Warnf("marshal msg error %s", err)
				return
			}

			marshaled = append(marshaled, []byte("\n")...)
			if _, err := self.writer.Write(marshaled); err != nil {
				log.Warnf("h2 writedata warning message %s\n", err)
				return
			}
			self.flusher.Flush()
		}
	}
}

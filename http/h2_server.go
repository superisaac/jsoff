package jsonzhttp

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/superisaac/jsonz"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"io"
	"net/http"
	"sync"
)

type H2Handler struct {
	Actor     *Actor
	serverCtx context.Context
	// options
	SpawnGoroutine bool
	UseH2C         bool

	fallbackHandler *H1Handler
	fallbackOnce    sync.Once
}

type H2Session struct {
	server      *H2Handler
	decoder     *json.Decoder
	writer      io.Writer
	flusher     http.Flusher
	httpRequest *http.Request
	rootCtx     context.Context
	done        chan error
	sendChannel chan jsonz.Message
	sessionId   string
}

func NewH2Handler(serverCtx context.Context, actor *Actor) *H2Handler {
	if actor == nil {
		actor = NewActor()
	}
	return &H2Handler{
		serverCtx:      serverCtx,
		Actor:          actor,
		SpawnGoroutine: true,
	}
}

func (self *H2Handler) H2CHandler() http.Handler {
	self.UseH2C = true
	h2server := &http2.Server{}
	return h2c.NewHandler(self, h2server)
}

func (self *H2Handler) FallbackHandler() *H1Handler {
	self.fallbackOnce.Do(func() {
		self.fallbackHandler = NewH1Handler(self.Actor)
	})
	return self.fallbackHandler
}

func (self *H2Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !self.UseH2C && !r.ProtoAtLeast(2, 0) {
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
	w.Write([]byte("{\"method\":\"hello\",\"params\":[]}\n"))
	flusher.Flush()

	decoder := json.NewDecoder(r.Body)
	session := &H2Session{
		server:      self,
		rootCtx:     r.Context(),
		httpRequest: r,
		writer:      w,
		flusher:     flusher,
		decoder:     decoder,
		done:        make(chan error, 10),
		sendChannel: make(chan jsonz.Message, 100),
		sessionId:   jsonz.NewUuid(),
	}
	defer func() {
		r.Body.Close()
		self.Actor.HandleClose(r, session)
	}()
	session.wait()
}

// websocket session
func (self *H2Session) wait() {
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

func (self *H2Session) recvLoop() {
	for {
		msg, err := jsonz.DecodeMessage(self.decoder)
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

func (self *H2Session) msgReceived(msg jsonz.Message) {
	req := NewRPCRequest(
		self.rootCtx,
		msg,
		TransportHTTP2,
		self.httpRequest)
	req.session = self

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

func (self *H2Session) Send(msg jsonz.Message) {
	self.sendChannel <- msg
}

func (self H2Session) SessionID() string {
	return self.sessionId
}

func (self *H2Session) sendLoop() {
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
			marshaled, err := jsonz.MessageBytes(msg)
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

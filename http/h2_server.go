package jsonzhttp

import (
	//"fmt"
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"net/http"
	"golang.org/x/net/http2"
)


type H2Handler struct {
	Actor     *Actor
	serverCtx context.Context
	// options
	SpawnGoroutine bool
}

type h2MsgFrame struct {
	Msg     jsonz.Message
	Frame   *http2.DataFrame
}

type H2Session struct {
	server      *H2Handler
	framer      *http2.Framer
	httpRequest *http.Request
	rootCtx     context.Context
	done        chan error
	sendChannel chan h2MsgFrame
	streamId    string
}

func NewH2Handler(serverCtx context.Context, actor *Actor) *H2Handler {
	if actor == nil {
		actor = NewActor()
	}
	return &H2Handler{
		serverCtx: serverCtx,
		Actor:     actor,
	}
}

func (self *H2Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	framer := http2.NewFramer(w, r.Body)

	defer func() {
		self.Actor.HandleClose(r)
	}()

	session := &H2Session{
		server:      self,
		rootCtx:     r.Context(),
		httpRequest: r,
		framer:      framer,
		done:        make(chan error, 10),
		sendChannel: make(chan h2MsgFrame, 100),
		streamId:    jsonz.NewUuid(),
	}
	session.wait()
	session.server = nil
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
	settings := map[http2.SettingID]uint32{}
	for {
		f, err1 := self.framer.ReadFrame()
		if err1 != nil {
			self.done <- errors.Wrap(err1, "framer.ReadFrame()")
			return
		}
		var err error
		switch ff := f.(type) {
		case *http2.DataFrame:
			if self.server.SpawnGoroutine {
				go self.msgBytesReceived(ff.Data(), ff)
			} else {
				self.msgBytesReceived(ff.Data(), ff)
			}
		case *http2.SettingsFrame:
			if !ff.IsAck() {
				ff.ForeachSetting(func(setting http2.Setting) error {
					settings[setting.ID] = setting.Val
					return nil
				})
				err = self.framer.WriteSettingsAck()
			}
		default:
			log.Infof("http2 frame type %+v", ff)
		}

		if err != nil {
			self.done <- errors.Wrap(err, "data framer")
			return
		}
	}
}

func (self *H2Session) msgBytesReceived(msgBytes []byte, src *http2.DataFrame) {
	msg, err := jsonz.ParseBytes(msgBytes)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", msgBytes)
		self.done <- errors.New("bad jsonrpc message")
		return
	}

	req := NewRPCRequest(
		self.rootCtx,
		msg,
		TransportHTTP2,
		self.httpRequest,
		self)
	req.streamId = self.streamId

	resmsg, err := self.server.Actor.Feed(req)
	if err != nil {
		self.done <- errors.Wrap(err, "actor.Feed")
		return
	}
	if resmsg != nil {
		frm := h2MsgFrame{Msg: resmsg, Frame: src}
		if resmsg.IsResultOrError() {
			self.sendChannel <- frm
		} else {
			self.Send(resmsg)
		}
	}
}

func (self *H2Session) Send(msg jsonz.Message) {
	self.sendChannel <- h2MsgFrame{Msg: msg, Frame: nil}
}

func (self *H2Session) sendLoop() {
	ctx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case frm, ok := <-self.sendChannel:
			if !ok {
				return
			}
			msg := frm.Msg
			if self.framer == nil {
				return
			}
			marshaled, err := jsonz.MessageBytes(msg)
			if err != nil {
				log.Warnf("marshal msg error %s", err)
				return
			}

			streamId := uint32(0)
			if frm.Frame != nil {
				streamId = frm.Frame.Header().StreamID
			}
			if err := self.framer.WriteData(streamId, true, marshaled); err != nil {
				log.Warnf("h2 writedata warning message %s", err)
				return
			}
		}
	}
}

package jsoffhttp

import (
	"context"
	"crypto/tls"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type pendingRequest struct {
	reqmsg        *jsoff.RequestMessage
	resultChannel chan jsoff.Message
	expire        time.Time
}

// errors
var TransportConnectFailed = errors.New("connect refused")
var TransportClosed = errors.New("streaming closed")

// the underline transport, currently there are websocket, h2 and net socket
// implementations
type Transport interface {
	Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error
	Close()
	Connected() bool
	ReadMessage() (msg jsoff.Message, readed bool, err error)
	WriteMessage(msg jsoff.Message) error
}

type StreamingClient struct {
	// the server url it connects to
	serverUrl *url.URL

	// extra http header taken to transports
	extraHeader http.Header

	// lock to prevent concurrent write
	connectLock sync.Mutex

	// jsonrpc request message pending for result
	pendingRequests sync.Map

	// on messsage handler
	messageHandler MessageHandler

	// on connected handler
	connectedHandler ConnectedHandler

	// on close handler
	closeHandler CloseHandler

	// func accompanied by context, this func is called when
	// client want to deliberatly close the connection
	cancelFunc func()

	// send channel to write messsage sequencially
	sendChannel chan jsoff.Message

	// channel to wait until connection closed
	closeChannel chan error

	// the underline transport adaptor in charge of read/write
	// bytes
	transport Transport

	// TLS settings
	clientTLS *tls.Config
}

func (self *StreamingClient) SetExtraHeader(h http.Header) {
	self.extraHeader = h
}

func (self *StreamingClient) IsStreaming() bool {
	return true
}

func (self *StreamingClient) SetClientTLSConfig(cfg *tls.Config) {
	self.clientTLS = cfg
}

func (self *StreamingClient) ClientTLSConfig() *tls.Config {
	return self.clientTLS
}

func (self *StreamingClient) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"server": self.serverUrl.String(),
	})
}

func (self *StreamingClient) ServerURL() *url.URL {
	return self.serverUrl
}

func (self *StreamingClient) InitStreaming(serverUrl *url.URL, transport Transport) {
	self.serverUrl = serverUrl
	self.transport = transport
	self.sendChannel = nil
	self.closeChannel = nil
}

func (self *StreamingClient) CloseChannel() chan error {
	return self.closeChannel
}

// wait connection close and return error
func (self *StreamingClient) Wait() error {
	if self.closeChannel != nil {
		err := <-self.closeChannel
		return err
	} else {
		// client not connected, just return
		return nil
	}
}

func (self *StreamingClient) Close() {
	if self.Connected() {
		self.Reset(nil)
	}
}

func (self *StreamingClient) Reset(err error) {
	if self.cancelFunc != nil {
		self.cancelFunc()
		self.cancelFunc = nil
	}

	if self.closeChannel != nil {
		self.closeChannel <- err
		self.closeChannel = nil
	}
	self.transport.Close()
	self.sendChannel = nil
}

func (self *StreamingClient) OnMessage(handler MessageHandler) error {
	if self.messageHandler != nil {
		return errors.New("message handler already exist!")
	}
	self.messageHandler = handler
	return nil
}

func (self *StreamingClient) OnConnected(handler ConnectedHandler) error {
	if self.connectedHandler != nil {
		return errors.New("connected handler already exist!")
	}
	self.connectedHandler = handler
	return nil
}

func (self *StreamingClient) OnClose(handler CloseHandler) error {
	if self.closeHandler != nil {
		return errors.New("close handler already exist!")
	}
	self.closeHandler = handler
	return nil
}

func (self *StreamingClient) Connect(rootCtx context.Context) error {
	self.connectLock.Lock()
	defer self.connectLock.Unlock()

	if !self.transport.Connected() {
		if err := self.transport.Connect(rootCtx, self.serverUrl, self.extraHeader); err != nil {
			//self.connectErr = err
			return err
		}
		if self.connectedHandler != nil {
			self.connectedHandler()
		}
		connCtx, cancel := context.WithCancel(rootCtx)
		self.cancelFunc = cancel
		self.sendChannel = make(chan jsoff.Message, 100)
		self.closeChannel = make(chan error, 10)
		go self.sendLoop(connCtx)
		go self.recvLoop()
	} else {
		self.Log().Debug("client already connected")
	}
	return nil
}

func (self *StreamingClient) handleError(err error) {
	if errors.Is(err, TransportClosed) {
		self.Log().Debug("transport closed")
	}
	self.Reset(err)
	if self.closeHandler != nil {
		self.closeHandler()
		self.closeHandler = nil
	}
}

func (self *StreamingClient) Connected() bool {
	return self.transport.Connected()
}

func (self *StreamingClient) sendLoop(connCtx context.Context) {
	//defer self.Reset(nil)
	defer func() {
		self.Log().Debug("sendLoop stop")
	}()
	ctx, cancel := context.WithCancel(connCtx)
	defer cancel()

	self.Log().Debug("sendLoop start")
	for {
		if !self.transport.Connected() {
			return
		}
		select {
		case <-ctx.Done():
			self.Log().Debug("ctx Done")
			self.Close()
			return
		case msg, ok := <-self.sendChannel:
			if !ok {
				return
			}
			if !self.transport.Connected() {
				return
			}
			err := self.transport.WriteMessage(msg)
			if err != nil {
				self.Log().Warnf("write msg error %s", err)
				self.handleError(err)
				return
			}
		}
	}
}

func (self *StreamingClient) recvLoop() {
	self.Log().Debug("recvLoop start")
	defer func() {
		self.Log().Debug("recvLoop stop")
	}()
	for {
		if !self.transport.Connected() {
			return
		}
		msg, readed, err := self.transport.ReadMessage()
		if err != nil {
			self.handleError(err)
			return
		}
		if !readed {
			continue
		}

		// assert msg != nil
		if !msg.IsResultOrError() {
			if self.messageHandler != nil {
				self.messageHandler(msg)
			} else {
				msg.Log().Debug("no message handler found")
			}
		} else {
			self.handleResult(msg)
		}
	}
}

func (self *StreamingClient) handleResult(msg jsoff.Message) {
	msgId := msg.MustId()
	v, loaded := self.pendingRequests.LoadAndDelete(msgId)
	if !loaded {
		if self.messageHandler != nil {
			self.messageHandler(msg)
		}
		return
	}

	if pending, ok := v.(*pendingRequest); ok {
		if msgId != pending.reqmsg.Id {
			resmsg := msg.ReplaceId(pending.reqmsg.Id)
			pending.resultChannel <- resmsg
		} else {
			pending.resultChannel <- msg
		}
	}
}

func (self *StreamingClient) expire(k interface{}, after time.Duration) {
	// ctx, cancel := context.WithCancel(rootCtx)
	// defer cancel()
	select {
	// case <- ctx.Done():
	// 	return
	case <-time.After(after):
		v, loaded := self.pendingRequests.LoadAndDelete(k)
		if loaded {
			if pending, ok := v.(*pendingRequest); ok {
				timeout := jsoff.ErrTimeout.ToMessage(pending.reqmsg)
				pending.resultChannel <- timeout
			}
		}
	}
}

func (self *StreamingClient) UnwrapCall(rootCtx context.Context, reqmsg *jsoff.RequestMessage, output interface{}) error {
	resmsg, err := self.Call(rootCtx, reqmsg)
	if err != nil {
		return err
	}
	if resmsg.IsResult() {
		err := jsoff.DecodeInterface(resmsg.MustResult(), output)
		if err != nil {
			return errors.Wrapf(err, "RPC(%s)", reqmsg.Method)
		}
		return nil
	} else {
		return resmsg.MustError()
	}
}

func (self *StreamingClient) Call(rootCtx context.Context, reqmsg *jsoff.RequestMessage) (jsoff.Message, error) {
	resmsg, err := self.request(rootCtx, reqmsg)
	if err != nil {
		return resmsg, errors.Wrapf(err, "RPC(%s)", reqmsg.Method)
	}
	return resmsg, nil
}

func (self *StreamingClient) request(rootCtx context.Context, reqmsg *jsoff.RequestMessage) (jsoff.Message, error) {
	err := self.Connect(rootCtx)
	if err != nil {
		return nil, err
	}
	ch := make(chan jsoff.Message, 10)

	sendmsg := reqmsg
	if _, loaded := self.pendingRequests.Load(reqmsg.Id); loaded {
		sendmsg = reqmsg.Clone(jsoff.NewUuid())
	}

	p := &pendingRequest{
		reqmsg:        reqmsg,
		resultChannel: ch,
		expire:        time.Now().Add(time.Second * 10),
	}
	self.pendingRequests.Store(sendmsg.Id, p)

	err = self.Send(rootCtx, sendmsg)
	if err != nil {
		return nil, err
	}
	go self.expire(sendmsg.Id, time.Second*10)

	resmsg, ok := <-ch
	if !ok {
		return nil, errors.New("result channel closed")
	}
	return resmsg, nil
}

func (self *StreamingClient) Send(rootCtx context.Context, msg jsoff.Message) error {
	err := self.Connect(rootCtx)
	if err != nil {
		return err
	}
	self.sendChannel <- msg
	return nil
}

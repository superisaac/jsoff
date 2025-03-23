package jsoffnet

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
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

func (client *StreamingClient) SetExtraHeader(h http.Header) {
	client.extraHeader = h
}

func (client *StreamingClient) IsStreaming() bool {
	return true
}

func (client *StreamingClient) SetClientTLSConfig(cfg *tls.Config) {
	client.clientTLS = cfg
}

func (client *StreamingClient) ClientTLSConfig() *tls.Config {
	return client.clientTLS
}

func (client *StreamingClient) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"server": client.serverUrl.String(),
	})
}

func (client *StreamingClient) ServerURL() *url.URL {
	return client.serverUrl
}

func (client *StreamingClient) InitStreaming(serverUrl *url.URL, transport Transport) {
	client.serverUrl = serverUrl
	client.transport = transport
	client.sendChannel = nil
	client.closeChannel = nil
}

func (client *StreamingClient) CloseChannel() chan error {
	return client.closeChannel
}

// wait connection close and return error
func (client *StreamingClient) Wait() error {
	if client.closeChannel != nil {
		err := <-client.closeChannel
		return err
	} else {
		// client not connected, just return
		return nil
	}
}

func (client *StreamingClient) Close() {
	if client.Connected() {
		client.Reset(nil)
	}
}

func (client *StreamingClient) Reset(err error) {
	if client.cancelFunc != nil {
		client.cancelFunc()
		client.cancelFunc = nil
	}

	if client.closeChannel != nil {
		client.closeChannel <- err
		client.closeChannel = nil
	}
	client.transport.Close()
	client.sendChannel = nil
}

func (client *StreamingClient) OnMessage(handler MessageHandler) error {
	if client.messageHandler != nil {
		return errors.New("message handler already exist!")
	}
	client.messageHandler = handler
	return nil
}

func (client *StreamingClient) OnConnected(handler ConnectedHandler) error {
	if client.connectedHandler != nil {
		return errors.New("connected handler already exist!")
	}
	client.connectedHandler = handler
	return nil
}

func (client *StreamingClient) OnClose(handler CloseHandler) error {
	if client.closeHandler != nil {
		return errors.New("close handler already exist!")
	}
	client.closeHandler = handler
	return nil
}

func (client *StreamingClient) Connect(rootCtx context.Context) error {
	client.connectLock.Lock()
	defer client.connectLock.Unlock()

	if !client.transport.Connected() {
		if err := client.transport.Connect(rootCtx, client.serverUrl, client.extraHeader); err != nil {
			//client.connectErr = err
			return err
		}
		if client.connectedHandler != nil {
			client.connectedHandler()
		}
		connCtx, cancel := context.WithCancel(rootCtx)
		client.cancelFunc = cancel
		client.sendChannel = make(chan jsoff.Message, 100)
		client.closeChannel = make(chan error, 10)
		go client.sendLoop(connCtx)
		go client.recvLoop()
	} else {
		client.Log().Debug("client already connected")
	}
	return nil
}

func (client *StreamingClient) handleError(err error) {
	if errors.Is(err, TransportClosed) {
		client.Log().Debug("transport closed")
	}
	client.Reset(err)
	if client.closeHandler != nil {
		client.closeHandler()
		client.closeHandler = nil
	}
}

func (client *StreamingClient) Connected() bool {
	return client.transport.Connected()
}

func (client *StreamingClient) sendLoop(connCtx context.Context) {
	//defer client.Reset(nil)
	defer func() {
		client.Log().Debug("sendLoop stop")
	}()
	ctx, cancel := context.WithCancel(connCtx)
	defer cancel()

	client.Log().Debug("sendLoop start")
	for {
		if !client.transport.Connected() {
			return
		}
		select {
		case <-ctx.Done():
			client.Log().Debug("ctx Done")
			client.Close()
			return
		case msg, ok := <-client.sendChannel:
			if !ok {
				return
			}
			if !client.transport.Connected() {
				return
			}
			err := client.transport.WriteMessage(msg)
			if err != nil {
				client.Log().Warnf("write msg error %s", err)
				client.handleError(err)
				return
			}
		}
	}
}

func (client *StreamingClient) recvLoop() {
	client.Log().Debug("recvLoop start")
	defer func() {
		client.Log().Debug("recvLoop stop")
	}()
	for {
		if !client.transport.Connected() {
			return
		}
		msg, readed, err := client.transport.ReadMessage()
		if err != nil {
			client.handleError(err)
			return
		}
		if !readed {
			continue
		}

		// assert msg != nil
		if !msg.IsResultOrError() {
			if client.messageHandler != nil {
				client.messageHandler(msg)
			} else {
				msg.Log().Debug("no message handler found")
			}
		} else {
			client.handleResult(msg)
		}
	}
}

func (client *StreamingClient) handleResult(msg jsoff.Message) {
	msgId := msg.MustId()
	v, loaded := client.pendingRequests.LoadAndDelete(msgId)
	if !loaded {
		if client.messageHandler != nil {
			client.messageHandler(msg)
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

func (client *StreamingClient) expire(k interface{}, after time.Duration) {
	time.Sleep(after)
	v, loaded := client.pendingRequests.LoadAndDelete(k)
	if loaded {
		if pending, ok := v.(*pendingRequest); ok {
			timeout := jsoff.ErrTimeout.ToMessage(pending.reqmsg)
			pending.resultChannel <- timeout
		}
	}
}

func (client *StreamingClient) UnwrapCall(rootCtx context.Context, reqmsg *jsoff.RequestMessage, output interface{}) error {
	resmsg, err := client.Call(rootCtx, reqmsg)
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

func (client *StreamingClient) Call(rootCtx context.Context, reqmsg *jsoff.RequestMessage) (jsoff.Message, error) {
	resmsg, err := client.request(rootCtx, reqmsg)
	if err != nil {
		return resmsg, errors.Wrapf(err, "RPC(%s)", reqmsg.Method)
	}
	return resmsg, nil
}

func (client *StreamingClient) request(rootCtx context.Context, reqmsg *jsoff.RequestMessage) (jsoff.Message, error) {
	err := client.Connect(rootCtx)
	if err != nil {
		return nil, err
	}
	ch := make(chan jsoff.Message, 10)

	sendmsg := reqmsg
	if _, loaded := client.pendingRequests.Load(reqmsg.Id); loaded {
		sendmsg = reqmsg.Clone(jsoff.NewUuid())
	}

	p := &pendingRequest{
		reqmsg:        reqmsg,
		resultChannel: ch,
		expire:        time.Now().Add(time.Second * 10),
	}
	client.pendingRequests.Store(sendmsg.Id, p)

	err = client.Send(rootCtx, sendmsg)
	if err != nil {
		return nil, err
	}
	go client.expire(sendmsg.Id, time.Second*10)
	if closeChannel := client.closeChannel; closeChannel != nil {
		select {
		case <-closeChannel:
			client.closeChannel = nil
			return nil, TransportClosed
		case resmsg, ok := <-ch:
			if !ok {
				return nil, errors.New("result channel closed")
			}
			return resmsg, nil
		}
	} else {
		resmsg, ok := <-ch
		if !ok {
			return nil, errors.New("result channel closed")
		}
		return resmsg, nil
	}
}

func (client *StreamingClient) Send(rootCtx context.Context, msg jsoff.Message) error {
	err := client.Connect(rootCtx)
	if err != nil {
		return err
	}
	client.sendChannel <- msg
	return nil
}

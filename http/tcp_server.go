package jsoffhttp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
	"io"
	"net"
)

// tcp session implements RPCSession
type TCPSession struct {
	server  *TCPServer
	decoder *json.Decoder
	//writer      io.Writer
	conn        net.Conn
	rootCtx     context.Context
	done        chan error
	sendChannel chan jsoff.Message
	sessionId   string
}

type TCPServer struct {
	Actor     *Actor
	serverCtx context.Context
}

func NewTCPServer(serverCtx context.Context, actor *Actor) *TCPServer {
	if actor == nil {
		actor = NewActor()
	}
	return &TCPServer{
		serverCtx: serverCtx,
		Actor:     actor,
	}
}

func (self *TCPServer) Start(rootCtx context.Context, bind string) error {
	listen, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}

	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Printf("accept failed, %v\n", err)
			continue
		}
		go self.processConnection(rootCtx, conn)
	}
}

func (self *TCPServer) processConnection(rootCtx context.Context, conn net.Conn) {
	decoder := json.NewDecoder(bufio.NewReader(conn))

	session := &TCPSession{
		server:      self,
		rootCtx:     rootCtx,
		conn:        conn,
		decoder:     decoder,
		done:        make(chan error, 10),
		sendChannel: make(chan jsoff.Message, 100),
		sessionId:   jsoff.NewUuid(),
	}
	defer func() {
		conn.Close()
		self.Actor.HandleClose(session)
	}()
	session.wait()
}

// tcp session methods
func (self *TCPSession) wait() {
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

func (self *TCPSession) recvLoop() {
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
		go self.msgReceived(msg)
	}
	// end of scanning
	self.done <- nil
	return
}

func (self *TCPSession) msgReceived(msg jsoff.Message) {
	req := NewRPCRequest(
		self.rootCtx,
		msg,
		TransportTCP).WithSession(self)

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

func (self *TCPSession) Send(msg jsoff.Message) {
	self.sendChannel <- msg
}

func (self TCPSession) SessionID() string {
	return self.sessionId
}

func (self *TCPSession) sendLoop() {
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
			if _, err := self.conn.Write(marshaled); err != nil {
				log.Warnf("tcp writedata warning message %v\n", err)
				return
			}
			//self.flusher.Flush()
		}
	}
}

package jsoffnet

import (
	"bufio"
	"context"
	"encoding/json"
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
	listener  net.Listener
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

func (s *TCPServer) Start(rootCtx context.Context, bind string) error {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}
	s.listener = listener

	for {
		if listener := s.listener; listener != nil {
			conn, err := listener.Accept()
			if err != nil {
				var opErr *net.OpError
				if errors.As(err, &opErr) {
					// tcp server stopped
					break
				} else {
					return errors.Wrap(err, "tcp.Accept")
				}
			}
			go s.processConnection(rootCtx, conn)
		} else {
			break
		}
	}
	return nil
}

func (s *TCPServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}
}

func (s *TCPServer) processConnection(rootCtx context.Context, conn net.Conn) {
	decoder := json.NewDecoder(bufio.NewReader(conn))

	session := &TCPSession{
		server:      s,
		rootCtx:     rootCtx,
		conn:        conn,
		decoder:     decoder,
		done:        make(chan error, 10),
		sendChannel: make(chan jsoff.Message, 100),
		sessionId:   jsoff.NewUuid(),
	}
	defer func() {
		conn.Close()
		s.Actor.HandleClose(session)
	}()
	session.wait()
}

// tcp session methods
func (session *TCPSession) wait() {
	connCtx, cancel := context.WithCancel(session.rootCtx)
	defer cancel()

	serverCtx, cancelServer := context.WithCancel(session.server.serverCtx)
	defer cancelServer()

	go session.sendLoop()
	go session.recvLoop()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-serverCtx.Done():
			return
		case err, ok := <-session.done:
			if ok && err != nil {
				log.Warnf("websocket error %s", err)
			}
			return
		}
	}
}

func (session *TCPSession) recvLoop() {
	for {
		msg, err := jsoff.DecodeMessage(session.decoder)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				session.done <- err
				return
			}
		}
		go session.msgReceived(msg)
	}
	// end of scanning
	session.done <- nil
}

func (session *TCPSession) msgReceived(msg jsoff.Message) {
	req := NewRPCRequest(
		session.rootCtx,
		msg,
		TransportTCP).WithSession(session)

	resmsg, err := session.server.Actor.Feed(req)
	if err != nil {
		session.done <- errors.Wrap(err, "actor.Feed")
		return
	}
	if resmsg != nil {
		if resmsg.IsResultOrError() {
			session.sendChannel <- resmsg
		} else {
			session.Send(resmsg)
		}
	}
}

func (session *TCPSession) Send(msg jsoff.Message) {
	session.sendChannel <- msg
}

func (session TCPSession) SessionID() string {
	return session.sessionId
}

func (session TCPSession) Context() context.Context {
	return session.rootCtx
}

func (session *TCPSession) sendLoop() {
	ctx, cancel := context.WithCancel(session.rootCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-session.sendChannel:
			if !ok {
				return
			}
			if session.decoder == nil {
				return
			}
			marshaled, err := jsoff.MessageBytes(msg)
			if err != nil {
				log.Warnf("marshal msg error %s", err)
				return
			}

			marshaled = append(marshaled, []byte("\n")...)
			if _, err := session.conn.Write(marshaled); err != nil {
				log.Warnf("tcp writedata warning message %v\n", err)
				return
			}
			//session.flusher.Flush()
		}
	}
}

package jsoffnet

import (
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/superisaac/jsoff"
	"io"
)

// http2 session
func (session *Http2Session) wait() {
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

func (session *Http2Session) recvLoop() {
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
		if session.server.SpawnGoroutine {
			go session.msgReceived(msg)
		} else {
			session.msgReceived(msg)
		}
	}
	// end of scanning
	session.done <- nil
}

func (session *Http2Session) msgReceived(msg jsoff.Message) {
	req := NewRPCRequest(
		session.rootCtx,
		msg,
		TransportHTTP2).WithHTTPRequest(session.httpRequest).WithSession(session)

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

func (session *Http2Session) Send(msg jsoff.Message) {
	session.sendChannel <- msg
}

func (session Http2Session) SessionID() string {
	return session.sessionId
}

func (session Http2Session) Context() context.Context {
	return session.rootCtx
}

func (session *Http2Session) sendLoop() {
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
			if _, err := session.writer.Write(marshaled); err != nil {
				log.Warnf("h2 writedata warning message %s\n", err)
				return
			}
			session.flusher.Flush()
		}
	}
}

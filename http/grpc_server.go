package jsonzhttp

import (
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"github.com/superisaac/jsonz/grpc"
	grpc "google.golang.org/grpc"
	"math"
	"net"
)

type GRPCHandler struct {
	jsonzgrpc.UnimplementedJSONZServer
	Actor     *Actor
	serverCtx context.Context
}

type GRPCSession struct {
	stream      jsonzgrpc.JSONZ_OpenStreamServer
	server      *GRPCHandler
	rootCtx     context.Context
	done        chan error
	sendChannel chan jsonz.Message
}

func NewGRPCHandler(serverCtx context.Context, actor *Actor) *GRPCHandler {
	if actor == nil {
		actor = NewActor()
	}
	return &GRPCHandler{
		Actor:     actor,
		serverCtx: serverCtx,
	}
}

/// grpc.Server implements http.Actor
func (self *GRPCHandler) ServerHandler(opts ...grpc.ServerOption) *grpc.Server {
	opts = append(opts,
		grpc.MaxConcurrentStreams(math.MaxUint32),
		grpc.WriteBufferSize(1024000),
		grpc.ReadBufferSize(1024000),
		grpc.ReadBufferSize(1024000),
	)
	handler := grpc.NewServer(opts...)
	jsonzgrpc.RegisterJSONZServer(handler, self)
	return handler

}

func (self *GRPCHandler) OpenStream(stream jsonzgrpc.JSONZ_OpenStreamServer) error {
	session := &GRPCSession{
		stream:      stream,
		server:      self,
		rootCtx:     stream.Context(),
		done:        make(chan error, 10),
		sendChannel: make(chan jsonz.Message, 100),
	}
	defer func() {
		session.server = nil
	}()
	return session.wait()
}

// gRPC session
func (self *GRPCSession) wait() error {
	connCtx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	serverCtx, cancelServer := context.WithCancel(self.server.serverCtx)
	defer cancelServer()

	go self.sendLoop()
	go self.recvLoop()

	for {
		select {
		case <-connCtx.Done():
			return nil
		case <-serverCtx.Done():
			return nil
		case err, ok := <-self.done:
			if !ok {
				log.Debugf("done received not ok")
				return nil
			} else if err != nil {
				log.Errorf("stream err %s", err)
				return err
			}
		}
	}
}

func (self *GRPCSession) recvLoop() {
	for {
		gmsg, err := self.stream.Recv()
		if err != nil {
			self.done <- errors.Wrap(err, "stream.Recv()")
			return
		}

		msg, err := jsonz.ParseBytes([]byte(gmsg.Body))
		if err != nil {
			self.done <- errors.Wrap(err, "jsonz.ParseBytes")
			return
		}
		self.msgReceived(msg)
	}
}

func (self *GRPCSession) msgReceived(msg jsonz.Message) {
	req := NewRPCRequest(
		self.rootCtx,
		msg,
		TransportGRPC,
		nil, // HttpRequest is nil
		self)
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

func (self *GRPCSession) Send(msg jsonz.Message) {
	self.sendChannel <- msg
}

func (self *GRPCSession) sendLoop() {
	ctx, cancel := context.WithCancel(self.rootCtx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-self.sendChannel:
			if !ok {
				self.done <- nil
				return
			}
			marshaled, err := jsonz.MessageBytes(msg)
			if err != nil {
				//log.Warnf("marshal msg error %s", err)
				self.done <- err
				return
			}

			gmsg := &jsonzgrpc.JSONRPCMessage{
				Body: marshaled,
			}

			if err := self.stream.Send(gmsg); err != nil {
				log.Warnf("write warning message %s", err)
				self.done <- err
				return
			}
		}
	}
}

// start grpc server
func GRPCServe(rootCtx context.Context, bind string, server *GRPCHandler, opts ...grpc.ServerOption) {
	lis, err := net.Listen("tcp", bind)
	if err != nil {
		log.Panicf("failed to listen: %v", err)
	} else {
		log.Debugf("entry server listen at %s", bind)
	}

	handler := server.ServerHandler(opts...)

	serverCtx, cancelServer := context.WithCancel(rootCtx)
	defer cancelServer()

	go func() {
		for {
			<-serverCtx.Done()
			log.Debugf("gRPC Server %s stops", bind)
			handler.Stop()
			return
		}
	}()

	//jsonzgrpc.RegisterJSONZServer(handler, server)
	handler.Serve(lis)
}

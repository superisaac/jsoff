package jsonzhttp

import (
	"context"
	"fmt"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	jsonzgrpc "github.com/superisaac/jsonz/grpc"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"io"
	"net/http"
	"net/url"
)

var (
	safeCodes = []codes.Code{codes.Unavailable, codes.Canceled}
)

// implements StreamingClient
type GRPCClient struct {
	StreamingClient
}

type gRPCTransport struct {
	client     *GRPCClient
	grpcClient jsonzgrpc.JSONZClient
	stream     jsonzgrpc.JSONZ_OpenStreamClient
}

func NewGRPCClient(serverUrl *url.URL) *GRPCClient {
	if serverUrl.Scheme != "h2c" && serverUrl.Scheme != "h2" {
		log.Panicf("server url %s is not grpc", serverUrl)
	}
	c := &GRPCClient{}
	transport := &gRPCTransport{client: c}
	c.InitStreaming(serverUrl, transport)
	return c
}

func (self *GRPCClient) String() string {
	return fmt.Sprintf("gRPC client %s", self.serverUrl)
}

// websocket transport methods
func (self *gRPCTransport) Close() {
	if self.stream != nil {
		//self.stream.Close()
		self.stream = nil
		self.grpcClient = nil
	}
}

func (self gRPCTransport) Connected() bool {
	return self.stream != nil
}

func (self *gRPCTransport) Connect(rootCtx context.Context, serverUrl *url.URL, headers ...http.Header) error {
	// headers is not used
	var opts []grpc.DialOption
	if serverUrl.Scheme == "h2c" {
		opts = append(opts, grpc.WithInsecure())
	} else if serverUrl.Scheme == "h2" {
		if cTLS := self.client.ClientTLSConfig(); cTLS != nil {
			creds := credentials.NewTLS(cTLS)
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}
		// TODO: credential settings
	} else {
		log.Panicf("invalid server url scheme %s", serverUrl.Scheme)
	}
	conn, err := grpc.Dial(serverUrl.Host, opts...)
	if err != nil {
		return errors.Wrap(err, "grpc.Dial()")
	}
	self.grpcClient = jsonzgrpc.NewJSONZClient(conn)

	stream, err := self.grpcClient.OpenStream(rootCtx, grpc_retry.WithMax(500))
	if err != nil {
		return err
	}
	self.stream = stream
	return nil
}

func (self *gRPCTransport) handleGRPCError(err error) error {
	if errors.Is(err, io.EOF) {
		log.Infof("cannot connect stream")
		return TransportClosed
	}
	code := grpc.Code(err)
	if code == codes.Unknown {
		cause := errors.Cause(err)
		if cause != nil {
			code = grpc.Code(cause)
		}
	}
	for _, safeCode := range safeCodes {
		if code == safeCode {
			log.Debugf("grpc connect code %d %s", code, code)
			return TransportClosed
		}
	}
	log.Warnf("error on handle %+v", err)
	return err
}

func (self *gRPCTransport) WriteMessage(msg jsonz.Message) error {
	marshaled, err := jsonz.MessageBytes(msg)
	if err != nil {
		return self.handleGRPCError(err)
	}

	gmsg := &jsonzgrpc.JSONRPCMessage{
		Body: marshaled,
	}

	if err := self.stream.Send(gmsg); err != nil {
		return err
	}
	return nil
}

func (self *gRPCTransport) ReadMessage() (jsonz.Message, bool, error) {
	gmsg, err := self.stream.Recv()
	if err != nil {
		return nil, false, self.handleGRPCError(err)
	}
	msg, err := jsonz.ParseBytes(gmsg.Body)
	if err != nil {
		log.Warnf("bad jsonrpc message %s", gmsg.Body)
		return nil, false, err
	}
	return msg, true, nil
}

package jsoffnet

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
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type TCPClient struct {
	StreamingClient
}

type tcpTransport struct {
	conn    net.Conn
	decoder *json.Decoder
	client  *TCPClient
}

func NewTCPClient(serverUrl *url.URL) *TCPClient {
	if serverUrl.Scheme != "tcp" {
		log.Panicf("server url %s is not tcp", serverUrl)
	}
	c := &TCPClient{}
	transport := &tcpTransport{client: c}
	c.InitStreaming(serverUrl, transport)
	return c
}

func (self *TCPClient) String() string {
	return fmt.Sprintf("tcp client %s", self.serverUrl)
}

// websocket transport methods
func (self *tcpTransport) Close() {
	if self.conn != nil {
		self.conn.Close()
		self.conn = nil
	}
}

func (self tcpTransport) Connected() bool {
	return self.conn != nil
}

func (self *tcpTransport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	conn, err := net.Dial("tcp", serverUrl.Host)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			self.client.Log().Infof("tcp operror %s", opErr)
			return TransportConnectFailed
		}
		return errors.Wrap(err, "tcp.connect")
	}
	self.conn = conn
	self.decoder = json.NewDecoder(bufio.NewReader(conn))
	return nil
}

func (self *tcpTransport) handleTCPError(err error) error {
	logger := self.client.Log()
	if errors.Is(err, io.EOF) {
		logger.Infof("tcp conn failed")
		return TransportClosed
	} else {
		logger.Warnf("tcp.ReadMessage error %s %s", reflect.TypeOf(err), err)
	}
	return errors.Wrap(err, "handleTCPError")
}

func (self *tcpTransport) WriteMessage(msg jsoff.Message) error {
	marshaled, err := jsoff.MessageBytes(msg)
	if err != nil {
		return err
	}

	if _, err := self.conn.Write(marshaled); err != nil {
		return self.handleTCPError(err)
	}
	return nil
}

func (self *tcpTransport) ReadMessage() (jsoff.Message, bool, error) {
	msg, err := jsoff.DecodeMessage(self.decoder)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, false, TransportClosed
		} else if strings.Contains(err.Error(), "read/write on closed pipe") {
			return nil, false, TransportClosed
		}
		self.client.Log().Warnf(
			"bad jsonrpc message %s %s, at pos %d",
			reflect.TypeOf(err), err, self.decoder.InputOffset())
		return nil, false, err
	}
	return msg, true, nil
}

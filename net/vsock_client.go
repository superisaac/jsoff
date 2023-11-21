package jsoffnet

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/mdlayher/vsock"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
)

type VsockClient struct {
	StreamingClient
}

type vsockTransport struct {
	conn    *vsock.Conn
	decoder *json.Decoder
	client  *VsockClient
}

func NewVsockClient(serverUrl *url.URL) *VsockClient {
	if serverUrl.Scheme != "vsock" {
		log.Panicf("server url %s is not vsock", serverUrl)
	}
	c := &VsockClient{}
	transport := &vsockTransport{client: c}
	c.InitStreaming(serverUrl, transport)
	return c
}

func (self *VsockClient) String() string {
	return fmt.Sprintf("vsock client %s", self.serverUrl)
}

// websocket transport methods
func (self *vsockTransport) Close() {
	if self.conn != nil {
		self.conn.Close()
		self.conn = nil
	}
}

func (self vsockTransport) Connected() bool {
	return self.conn != nil
}

func (self *vsockTransport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	// serverUrl is in the form of "vsock://<contextId>:<port>"
	contextID, err := strconv.ParseUint(serverUrl.Hostname(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "vsock.parseContextId")
	}
	port, err := strconv.ParseUint(serverUrl.Port(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "vsock.parsePort")
	}
	conn, err := vsock.Dial(uint32(contextID), uint32(port), nil)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			self.client.Log().Infof("vsock operror %s", opErr)
			return TransportConnectFailed
		}
		return errors.Wrap(err, "vsock.connect")
	}
	self.conn = conn
	self.decoder = json.NewDecoder(bufio.NewReader(conn))
	return nil
}

func (self *vsockTransport) handleTCPError(err error) error {
	logger := self.client.Log()
	if errors.Is(err, io.EOF) {
		logger.Infof("vsock conn failed")
		return TransportClosed
	} else {
		logger.Warnf("vsock.ReadMessage error %s %s", reflect.TypeOf(err), err)
	}
	return errors.Wrap(err, "handleTCPError")
}

func (self *vsockTransport) WriteMessage(msg jsoff.Message) error {
	marshaled, err := jsoff.MessageBytes(msg)
	if err != nil {
		return err
	}

	if _, err := self.conn.Write(marshaled); err != nil {
		return self.handleTCPError(err)
	}
	return nil
}

func (self *vsockTransport) ReadMessage() (jsoff.Message, bool, error) {
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

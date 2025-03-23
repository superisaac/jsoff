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

func (client *VsockClient) String() string {
	return fmt.Sprintf("vsock client %s", client.serverUrl)
}

// websocket transport methods
func (t *vsockTransport) Close() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

func (t vsockTransport) Connected() bool {
	return t.conn != nil
}

func (t *vsockTransport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
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
			t.client.Log().Infof("vsock operror %s", opErr)
			return TransportConnectFailed
		}
		return errors.Wrap(err, "vsock.connect")
	}
	t.conn = conn
	t.decoder = json.NewDecoder(bufio.NewReader(conn))
	return nil
}

func (t *vsockTransport) handleTCPError(err error) error {
	logger := t.client.Log()
	if errors.Is(err, io.EOF) {
		logger.Infof("vsock conn failed")
		return TransportClosed
	} else {
		logger.Warnf("vsock.ReadMessage error %s %s", reflect.TypeOf(err), err)
	}
	return errors.Wrap(err, "handleTCPError")
}

func (t *vsockTransport) WriteMessage(msg jsoff.Message) error {
	marshaled, err := jsoff.MessageBytes(msg)
	if err != nil {
		return err
	}

	if _, err := t.conn.Write(marshaled); err != nil {
		return t.handleTCPError(err)
	}
	return nil
}

func (t *vsockTransport) ReadMessage() (jsoff.Message, bool, error) {
	msg, err := jsoff.DecodeMessage(t.decoder)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, false, TransportClosed
		} else if strings.Contains(err.Error(), "read/write on closed pipe") {
			return nil, false, TransportClosed
		}
		t.client.Log().Warnf(
			"bad jsonrpc message %s %s, at pos %d",
			reflect.TypeOf(err), err, t.decoder.InputOffset())
		return nil, false, err
	}
	return msg, true, nil
}

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

func (client *TCPClient) String() string {
	return fmt.Sprintf("tcp client %s", client.serverUrl)
}

// websocket transport methods
func (t *tcpTransport) Close() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

func (t tcpTransport) Connected() bool {
	return t.conn != nil
}

func (t *tcpTransport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	conn, err := net.Dial("tcp", serverUrl.Host)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			t.client.Log().Infof("tcp operror %s", opErr)
			return TransportConnectFailed
		}
		return errors.Wrap(err, "tcp.connect")
	}
	t.conn = conn
	t.decoder = json.NewDecoder(bufio.NewReader(conn))
	return nil
}

func (t *tcpTransport) handleTCPError(err error) error {
	logger := t.client.Log()
	if errors.Is(err, io.EOF) {
		logger.Infof("tcp conn failed")
		return TransportClosed
	} else {
		logger.Warnf("tcp.ReadMessage error %s %s", reflect.TypeOf(err), err)
	}
	return errors.Wrap(err, "handleTCPError")
}

func (t *tcpTransport) WriteMessage(msg jsoff.Message) error {
	marshaled, err := jsoff.MessageBytes(msg)
	if err != nil {
		return err
	}

	if _, err := t.conn.Write(marshaled); err != nil {
		return t.handleTCPError(err)
	}
	return nil
}

func (t *tcpTransport) ReadMessage() (jsoff.Message, bool, error) {
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

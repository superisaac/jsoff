package jsoffnet

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
	"golang.org/x/net/http2"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
)

type Http2Client struct {
	StreamingClient

	httpClient *http.Client
	clientOnce sync.Once

	// use h2c
	UseHttp2C bool
}

type h2Transport struct {
	client  *Http2Client
	resp    *http.Response
	decoder *json.Decoder
	writer  io.Writer
	flusher http.Flusher
}

func NewHttp2Client(serverUrl *url.URL) *Http2Client {
	newUrl, err := url.Parse(serverUrl.String())
	useh2c := false
	if err != nil {
		log.Panicf("copy url error %s", err)
	}
	if newUrl.Scheme == "h2" {
		newUrl.Scheme = "https"
	} else if newUrl.Scheme == "h2c" {
		newUrl.Scheme = "http"
		useh2c = true
	}
	if newUrl.Scheme != "https" && newUrl.Scheme != "http" {
		log.Panicf("server url %s is not http2", serverUrl)
	}
	c := &Http2Client{UseHttp2C: useh2c}
	transport := &h2Transport{client: c}
	c.InitStreaming(newUrl, transport)
	return c
}

func (t *Http2Client) HTTPClient() *http.Client {
	t.clientOnce.Do(func() {
		if t.UseHttp2C {
			// refer to https://www.mailgun.com/blog/http-2-cleartext-h2c-client-example-go/
			trans := &http2.Transport{
				AllowHTTP: true,
				// Pretend we are dialing a TLS endpoint.
				// Note, we ignore the passed tls.Config
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			}
			t.httpClient = &http.Client{
				Transport: trans,
			}
		} else {
			trans := &http2.Transport{
				AllowHTTP: true,
				//WriteByteTimeout: time.Second * 15,
				TLSClientConfig: t.ClientTLSConfig(),
			}

			t.httpClient = &http.Client{
				Transport: trans,
			}
		}
	})
	return t.httpClient
}

func (client *Http2Client) String() string {
	return fmt.Sprintf("http2 client %s", client.serverUrl)
}

// http2 transport methods
func (t *h2Transport) Close() {
	if t.resp != nil {
		t.resp.Body.Close()
		t.resp = nil
		t.writer = nil
		t.flusher = nil
		//self.decoder = nil
	}
}

func (t h2Transport) Connected() bool {
	return t.resp != nil
}

func (t *h2Transport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	pipeReader, pipeWriter := io.Pipe()

	req := &http.Request{
		Method: "PRI",
		URL:    serverUrl,
		Header: header,
		Body:   pipeReader,
	}

	resp, err := t.client.HTTPClient().Do(req)
	if err != nil {
		return t.handleHttp2Error(err)
	}
	t.writer = pipeWriter
	t.resp = resp
	t.decoder = json.NewDecoder(resp.Body)
	return nil
}

func (t *h2Transport) handleHttp2Error(err error) error {
	logger := t.client.Log()
	var urlErr *url.Error
	if errors.Is(err, io.EOF) {
		logger.Debugf("h2 conn failed")
		return TransportClosed
	} else if errors.As(err, &urlErr) {
		logger.Debugf("h2 conn url.Error")
		return TransportClosed
	} else {
		logger.Warnf("transport error %s %s", reflect.TypeOf(err), err)
	}
	return errors.Wrap(err, "h2transport.handleHttp2Error")
}

func (t *h2Transport) WriteMessage(msg jsoff.Message) error {
	marshaled, err := jsoff.MessageBytes(msg)
	if err != nil {
		return err
	}

	marshaled = append(marshaled, []byte("\n")...)
	if _, err := t.writer.Write(marshaled); err != nil {
		return t.handleHttp2Error(err)
	}
	return nil
}

func (t *h2Transport) ReadMessage() (jsoff.Message, bool, error) {
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

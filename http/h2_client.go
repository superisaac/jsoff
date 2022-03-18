package jsonzhttp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"golang.org/x/net/http2"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
)

type H2Client struct {
	StreamingClient

	httpClient *http.Client
	clientOnce sync.Once

	// use h2c
	UseH2C bool
}

type h2Transport struct {
	client  *H2Client
	resp    *http.Response
	decoder *json.Decoder
	writer  io.Writer
	flusher http.Flusher
}

func NewH2Client(serverUrl *url.URL) *H2Client {
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
	c := &H2Client{UseH2C: useh2c}
	transport := &h2Transport{client: c}
	c.InitStreaming(newUrl, transport)
	return c
}

func (self *H2Client) HTTPClient() *http.Client {
	self.clientOnce.Do(func() {
		if self.UseH2C {
			// refer to https://www.mailgun.com/blog/http-2-cleartext-h2c-client-example-go/
			trans := &http2.Transport{
				AllowHTTP: true,
				// Pretend we are dialing a TLS endpoint.
				// Note, we ignore the passed tls.Config
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			}
			self.httpClient = &http.Client{
				Transport: trans,
			}
		} else {
			trans := &http2.Transport{
				AllowHTTP: true,
				//WriteByteTimeout: time.Second * 15,
				TLSClientConfig: self.ClientTLSConfig(),
			}

			self.httpClient = &http.Client{
				Transport: trans,
			}
		}
	})
	return self.httpClient
}

func (self *H2Client) String() string {
	return fmt.Sprintf("http2 client %s", self.serverUrl)
}

// http2 transport methods
func (self *h2Transport) Close() {
	if self.resp != nil {
		self.resp.Body.Close()
		self.resp = nil
		self.writer = nil
		self.flusher = nil
		self.decoder = nil
	}
}

func (self h2Transport) Connected() bool {
	return self.resp != nil
}

func (self *h2Transport) Connect(rootCtx context.Context, serverUrl *url.URL, header http.Header) error {
	pipeReader, pipeWriter := io.Pipe()

	req := &http.Request{
		Method: "POST",
		URL:    serverUrl,
		Header: header,
		Body:   pipeReader,
	}

	resp, err := self.client.HTTPClient().Do(req)
	if err != nil {
		return self.handleTransportError(err)
	}
	self.writer = pipeWriter
	self.resp = resp
	self.decoder = json.NewDecoder(resp.Body)
	return nil
}

func (self *h2Transport) handleTransportError(err error) error {
	logger := self.client.Log()
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
	return err
}

func (self *h2Transport) WriteMessage(msg jsonz.Message) error {
	marshaled, err := jsonz.MessageBytes(msg)
	if err != nil {
		return err
	}

	marshaled = append(marshaled, []byte("\n")...)
	if _, err := self.writer.Write(marshaled); err != nil {
		return self.handleTransportError(err)
	}
	return nil
}

func (self *h2Transport) ReadMessage() (jsonz.Message, bool, error) {
	msg, err := jsonz.DecodeMessage(self.decoder)
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

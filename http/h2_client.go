package jsonzhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"golang.org/x/net/http2"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sync"
)

type H2Client struct {
	StreamingClient
	httpClient *http.Client
	clientOnce sync.Once
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
	if err != nil {
		log.Panicf("copy url error %s", err)
	}
	if newUrl.Scheme == "h2" {
		newUrl.Scheme = "https"
	} else if newUrl.Scheme == "h2c" {
		newUrl.Scheme = "http"
	}
	if newUrl.Scheme != "https" && newUrl.Scheme != "http" {
		log.Panicf("server url %s is not http2", serverUrl)
	}
	c := &H2Client{}
	transport := &h2Transport{client: c}
	c.InitStreaming(newUrl, transport)
	return c
}

func (self *H2Client) HTTPClient() *http.Client {
	self.clientOnce.Do(func() {
		self.httpClient = &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				//WriteByteTimeout: time.Second * 15,
				TLSClientConfig: self.ClientTLSConfig(),
			},
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
		return err
	}
	self.writer = pipeWriter
	self.resp = resp
	self.decoder = json.NewDecoder(resp.Body)
	return nil
}

func (self *h2Transport) handleTransportError(err error) error {
	logger := self.client.Log()
	if errors.Is(err, io.EOF) {
		logger.Infof("websocket conn failed")
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
		}
		self.client.Log().Warnf("bad jsonrpc message at pos %d", self.decoder.InputOffset())
		return nil, false, err
	}
	return msg, true, nil
}

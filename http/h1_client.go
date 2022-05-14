package jlibhttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jlib"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type H1Client struct {
	serverUrl   *url.URL
	extraHeader http.Header
	httpClient  *http.Client

	connectOnce sync.Once

	clientTLS *tls.Config
}

func NewH1Client(serverUrl *url.URL) *H1Client {
	if serverUrl.Scheme != "http" && serverUrl.Scheme != "https" {
		log.Panicf("server url %s is not http", serverUrl)
	}
	return &H1Client{serverUrl: serverUrl}
}

func (self *H1Client) ServerURL() *url.URL {
	return self.serverUrl
}

func (self *H1Client) connect() {
	self.connectOnce.Do(func() {
		tr := &http.Transport{
			MaxIdleConns:        30,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		}
		if self.clientTLS != nil {
			tr.TLSClientConfig = self.clientTLS
		}
		self.httpClient = &http.Client{
			Transport: tr,
			Timeout:   5 * time.Second,
		}
	})
}

func (self *H1Client) SetExtraHeader(h http.Header) {
	self.extraHeader = h
}
func (self *H1Client) SetClientTLSConfig(cfg *tls.Config) {
	self.clientTLS = cfg
}

func (self *H1Client) UnwrapCall(rootCtx context.Context, reqmsg *jlib.RequestMessage, output interface{}) error {
	resmsg, err := self.Call(rootCtx, reqmsg)
	if err != nil {
		return err
	}
	if resmsg.IsResult() {
		err := jlib.DecodeInterface(resmsg.MustResult(), output)
		if err != nil {
			return errors.Wrapf(err, "RPC(%s)", reqmsg.Method)
		}
		return nil
	} else {
		return resmsg.MustError()
	}
}

func (self *H1Client) Call(rootCtx context.Context, reqmsg *jlib.RequestMessage) (jlib.Message, error) {
	resmsg, err := self.request(rootCtx, reqmsg)
	if err != nil {
		return resmsg, errors.Wrapf(err, "RPC(%s)", reqmsg.Method)
	}
	return resmsg, nil
}

func (self *H1Client) request(rootCtx context.Context, reqmsg *jlib.RequestMessage) (jlib.Message, error) {
	self.connect()

	traceId := reqmsg.TraceId()

	reqmsg.SetTraceId("")

	marshaled, err := jlib.MessageBytes(reqmsg)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", self.serverUrl.String(), reader)
	if err != nil {
		return nil, errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if self.extraHeader != nil {
		for k, vs := range self.extraHeader {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	resp, err := self.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		abnResp := &WrappedResponse{
			Response: resp,
		}
		return nil, errors.Wrap(abnResp, "abnormal response")
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}
	respmsg, err := jlib.ParseBytes(respBody)
	if err != nil {
		return nil, err
	}
	respmsg.SetTraceId(traceId)
	return respmsg, nil
}

func (self *H1Client) Send(rootCtx context.Context, msg jlib.Message) error {
	self.connect()

	traceId := msg.TraceId()
	msg.SetTraceId("")

	marshaled, err := jlib.MessageBytes(msg)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", self.serverUrl.String(), reader)
	if err != nil {
		return errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if self.extraHeader != nil {
		for k, vs := range self.extraHeader {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	resp, err := self.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		abnResp := &WrappedResponse{
			Response: resp,
		}
		return errors.Wrap(abnResp, "abnormal response")
	}
	return nil
}

func (self *H1Client) IsStreaming() bool {
	return false
}

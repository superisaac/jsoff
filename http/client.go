package jsonzhttp

import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonz"
	"io/ioutil"
	"net/http"
	"time"
)

type HTTPClient struct {
	serverUrl  string
	httpClient *http.Client
}

func NewHTTPClient(serverUrl string) *HTTPClient {
	return &HTTPClient{serverUrl: serverUrl}
}

func (self *HTTPClient) connect() {
	if self.httpClient == nil {
		tr := &http.Transport{
			MaxIdleConns:        30,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		}
		self.httpClient = &http.Client{
			Transport: tr,
			Timeout:   5 * time.Second,
		}
	}
}

func (self *HTTPClient) Call(rootCtx context.Context, reqmsg *jsonz.RequestMessage) (jsonz.Message, error) {
	self.connect()

	traceId := reqmsg.TraceId()

	reqmsg.SetTraceId("")

	marshaled, err := jsonz.MessageBytes(reqmsg)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", self.serverUrl, reader)
	if err != nil {
		return nil, errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := self.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		abnResp := &UpstreamResponse{
			Response: resp,
		}
		return nil, errors.Wrap(abnResp, "abnormal response")
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}
	respmsg, err := jsonz.ParseBytes(respBody)
	if err != nil {
		return nil, err
	}
	respmsg.SetTraceId(traceId)
	return respmsg, nil
}

func (self *HTTPClient) Send(rootCtx context.Context, msg jsonz.Message) error {
	self.connect()

	traceId := msg.TraceId()
	msg.SetTraceId("")

	marshaled, err := jsonz.MessageBytes(msg)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", self.serverUrl, reader)
	if err != nil {
		return errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := self.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		abnResp := &UpstreamResponse{
			Response: resp,
		}
		return errors.Wrap(abnResp, "abnormal response")
	}
	return nil
}

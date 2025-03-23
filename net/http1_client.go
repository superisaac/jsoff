package jsoffnet

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
)

type Http1Client struct {
	serverUrl     *url.URL
	extraHeader   http.Header
	httpClient    *http.Client
	clientOptions ClientOptions

	connectOnce sync.Once

	clientTLS *tls.Config
}

func NewHttp1Client(serverUrl *url.URL, optlist ...ClientOptions) *Http1Client {
	if serverUrl.Scheme != "http" && serverUrl.Scheme != "https" {
		log.Panicf("server url %s is not http", serverUrl)
	}

	clientOptions := ClientOptions{}
	if len(optlist) > 0 {
		clientOptions = optlist[0]
	}
	return &Http1Client{serverUrl: serverUrl, clientOptions: clientOptions}
}

func (client *Http1Client) ServerURL() *url.URL {
	return client.serverUrl
}

func (client *Http1Client) connect() {
	client.connectOnce.Do(func() {
		timeout := client.clientOptions.Timeout
		if timeout <= 0 {
			timeout = 5
		}
		tr := &http.Transport{
			MaxIdleConns:        30,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		}
		if client.clientTLS != nil {
			tr.TLSClientConfig = client.clientTLS
		}
		client.httpClient = &http.Client{
			Transport: tr,
			Timeout:   time.Duration(timeout) * time.Second,
		}
	})
}

func (client *Http1Client) SetExtraHeader(h http.Header) {
	client.extraHeader = h
}
func (client *Http1Client) SetClientTLSConfig(cfg *tls.Config) {
	client.clientTLS = cfg
}

func (self *Http1Client) UnwrapCall(rootCtx context.Context, reqmsg *jsoff.RequestMessage, output interface{}) error {
	resmsg, err := self.Call(rootCtx, reqmsg)
	if err != nil {
		return err
	}
	if resmsg.IsResult() {
		err := jsoff.DecodeInterface(resmsg.MustResult(), output)
		if err != nil {
			return errors.Wrapf(err, "RPC(%s)", reqmsg.Method)
		}
		return nil
	} else {
		return resmsg.MustError()
	}
}

func (client *Http1Client) Call(rootCtx context.Context, reqmsg *jsoff.RequestMessage) (jsoff.Message, error) {
	resmsg, err := client.request(rootCtx, reqmsg)
	if err != nil {
		return resmsg, errors.Wrapf(err, "RPC(%s)", reqmsg.Method)
	}
	return resmsg, nil
}

func (client *Http1Client) request(rootCtx context.Context, reqmsg *jsoff.RequestMessage) (jsoff.Message, error) {
	client.connect()

	traceId := reqmsg.TraceId()

	reqmsg.SetTraceId("")

	marshaled, err := jsoff.MessageBytes(reqmsg)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", client.serverUrl.String(), reader)
	if err != nil {
		return nil, errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if client.extraHeader != nil {
		for k, vs := range client.extraHeader {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			timeoutResp := &SimpleResponse{
				Code: http.StatusRequestTimeout,
				Body: []byte(`"request timeout"`),
			}
			return nil, errors.Wrap(timeoutResp, "request timeout")
		}
		return nil, errors.Wrap(err, "http Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buffer bytes.Buffer
		readed, err := buffer.ReadFrom(resp.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "read from response, readed=%d, status=%d", readed, resp.StatusCode)
		}
		// TODO: handle ErrTooLarge
		abnResp := &WrappedResponse{
			Response: resp,
			Body:     buffer.Bytes(),
		}
		reqmsg.Log().WithFields(log.Fields{
			"server": client.serverUrl.String(),
			"status": resp.StatusCode,
		}).Warnf("abnormal response")
		return nil, errors.Wrap(abnResp, "abnormal response")
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}
	respmsg, err := jsoff.ParseBytes(respBody)
	if err != nil {
		return nil, err
	}
	if responseMsg, ok := respmsg.(jsoff.ResponseMessage); ok {
		for header, values := range resp.Header {
			if strings.HasPrefix(strings.ToUpper(header), "X-") {
				for _, value := range values {
					responseMsg.ResponseHeader().Add(header, value)
				}
			}
		}
	}
	respmsg.SetTraceId(traceId)
	return respmsg, nil
}

func (client *Http1Client) Send(rootCtx context.Context, msg jsoff.Message) error {
	client.connect()

	traceId := msg.TraceId()
	msg.SetTraceId("")

	marshaled, err := jsoff.MessageBytes(msg)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(marshaled)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", client.serverUrl.String(), reader)
	if err != nil {
		return errors.Wrap(err, "http.NewRequestWithContext")
	}
	if traceId != "" {
		req.Header.Add("X-Trace-Id", traceId)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if client.extraHeader != nil {
		for k, vs := range client.extraHeader {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	resp, err := client.httpClient.Do(req)
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

func (client *Http1Client) IsStreaming() bool {
	return false
}

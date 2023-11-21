package jsoffnet

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/superisaac/jsoff"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func urlParse(server string) *url.URL {
	u, err := url.Parse(server)
	if err != nil {
		panic(err)
	}
	return u
}

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

const (
	addSchema = `{
  "type": "method",
  "params": ["integer", {"type": "integer"}]
}`

	addSchemaYaml = `
---
type: method
params:
  - integer
  - type: integer
`
)

func serverTLS() *TLSConfig {
	return &TLSConfig{
		Certfile: "testdata/localhost.crt",
		Keyfile:  "testdata/localhost.key",
	}
}

func clientTLS() *tls.Config {
	// client certificates using CA
	cacert, err := os.ReadFile("testdata/ca.crt")
	if err != nil {
		panic(err)
	}
	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cacert)
	return &tls.Config{
		RootCAs:            certpool,
		InsecureSkipVerify: true,
	}
}

func TestServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH1Handler(nil)
	server.Actor.On("echo", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28000", server)
	time.Sleep(10 * time.Millisecond)

	// request a GET method
	resp, err := http.Get("http://127.0.0.1:28000")
	assert.Equal(http.StatusMethodNotAllowed, resp.StatusCode)
	respData, _ := io.ReadAll(resp.Body)
	assert.Equal("Method not allowed", string(respData))

	client := NewH1Client(urlParse("http://127.0.0.1:28000"))

	// right request
	params := [](interface{}){"hello001"}
	reqmsg := jsoff.NewRequestMessage(1, "echo", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello001", res)

	// method not found
	params1 := [](interface{}){"hello002"}
	reqmsg1 := jsoff.NewRequestMessage(666, "echoxxx", params1)
	resmsg1, err := client.Call(rootCtx, reqmsg1)
	assert.Nil(err)
	assert.True(resmsg1.IsError())
	errbody := resmsg1.MustError()
	assert.Equal(jsoff.ErrMethodNotFound.Code, errbody.Code)
}

func TestMissing(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH1Handler(nil)
	err := server.Actor.OnMissing(func(req *RPCRequest) (interface{}, error) {
		msg := req.Msg()
		assert.True(msg.IsNotify())
		assert.Equal("testnotify", msg.MustMethod())
		return nil, nil
	})
	assert.Nil(err)

	go ListenAndServe(rootCtx, "127.0.0.1:28003", server)
	time.Sleep(10 * time.Millisecond)

	client := NewH1Client(urlParse("http://127.0.0.1:28003"))
	// right request
	params := [](interface{}){"hello003"}
	ntfmsg := jsoff.NewNotifyMessage("testnotify", params)

	err = client.Send(rootCtx, ntfmsg)
	assert.Nil(err)
}

func TestTypedServerClient(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH1Handler(nil)
	err := server.Actor.OnTypedRequest("wrongArg", func(a int, b int) (int, error) {
		return a + b, nil
	})
	assert.NotNil(err)
	assert.Equal("the first arg must be *jsoffnet.RPCRequest", err.Error())

	err = server.Actor.OnTypedContext("wrongNoContext", func(a int, b int) (int, error) {
		return a + b, nil
	})
	assert.NotNil(err)
	assert.Equal("the first arg must be context.Context", err.Error())

	server.Actor.OnTypedContext("echoTyped", func(ctx context.Context, v string) (string, error) {
		return v, nil
	})

	server.Actor.OnTypedContext("add", func(ctx context.Context, a, b int) (int, error) {
		return a + b, nil
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28001", server)
	time.Sleep(10 * time.Millisecond)

	client := NewH1Client(urlParse("http://127.0.0.1:28001"))

	// right request
	params := [](interface{}){"hello004"}
	reqmsg := jsoff.NewRequestMessage(1, "echoTyped", params)

	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.True(resmsg.IsResult())
	res := resmsg.MustResult()
	assert.Equal("hello004", res)

	// type mismatch
	params1 := [](interface{}){true}
	reqmsg1 := jsoff.NewRequestMessage(1, "echoTyped", params1)

	resmsg1, err1 := client.Call(rootCtx, reqmsg1)
	assert.Nil(err1)
	assert.True(resmsg1.IsError())
	errbody1 := resmsg1.MustError()
	assert.Equal(-32602, errbody1.Code) // params error
	assert.True(strings.Contains(errbody1.Message, "got unconvertible type"))
	// test params size
	params2 := [](interface{}){}
	reqmsg2 := jsoff.NewRequestMessage(2, "echoTyped", params2)

	resmsg2, err2 := client.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.True(resmsg2.IsError())
	errbody2 := resmsg2.MustError()
	assert.Equal(-32602, errbody2.Code)
	assert.Equal("no enough params size", errbody2.Message)

	// test add 2 numbers
	params3 := [](interface{}){6, 3}
	reqmsg3 := jsoff.NewRequestMessage(3, "add", params3)
	resmsg3, err3 := client.Call(rootCtx, reqmsg3)
	assert.Nil(err3)
	assert.True(resmsg3.IsResult())
	res3 := resmsg3.MustResult()
	assert.Equal(json.Number("9"), res3)

	// test add 2 numbers with typing mismatch
	params4 := [](interface{}){"6", 4}
	reqmsg4 := jsoff.NewRequestMessage(4, "add", params4)
	resmsg4, err4 := client.Call(rootCtx, reqmsg4)
	assert.Nil(err4)
	assert.True(resmsg4.IsError())
	errbody4 := resmsg4.MustError()
	assert.Equal(-32602, errbody4.Code)
	assert.True(strings.Contains(errbody4.Message, "got unconvertible type"))

	// test add 2 numbers with typing mismatch
	params5 := [](interface{}){"6", 5}
	reqmsg5 := jsoff.NewRequestMessage(5, "add", params5)
	var res5 int
	err5 := client.UnwrapCall(rootCtx, reqmsg5, &res5)
	assert.NotNil(err5)
	var errbody5 *jsoff.RPCError
	assert.True(errors.As(err5, &errbody5))
	assert.Equal(-32602, errbody5.Code)
	assert.True(strings.Contains(errbody5.Message, "got unconvertible type"))

	// correct unwrapcall
	params6 := [](interface{}){8, 99}
	reqmsg6 := jsoff.NewRequestMessage(6, "add", params6)
	var res6 int
	err6 := client.UnwrapCall(rootCtx, reqmsg6, &res6)
	assert.Nil(err6)
	assert.Equal(107, res6)
}

func TestHandlerSchema(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH1Handler(nil)
	server.Actor.ValidateSchema = true
	server.Actor.On("add2num", func(params []interface{}) (interface{}, error) {
		var tp struct {
			A int
			B int
		}
		err := jsoff.DecodeParams(params, &tp)
		if err != nil {
			return nil, err
		}
		return tp.A + tp.B, nil
	}, WithSchemaJson(addSchema))

	go ListenAndServe(rootCtx, "127.0.0.1:28040", server)
	time.Sleep(10 * time.Millisecond)

	client := NewH1Client(urlParse("http://127.0.0.1:28040"))

	// right request
	reqmsg := jsoff.NewRequestMessage(
		1, "add2num", []interface{}{5, 8})
	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.Equal(json.Number("13"), resmsg.MustResult())

	reqmsg2 := jsoff.NewRequestMessage(
		2, "add2num", []interface{}{"12", "a str"})
	resmsg2, err2 := client.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.Equal(jsoff.ErrInvalidSchema.Code, resmsg2.MustError().Code)
	assert.Equal("Validation Error: .params[0] data is not integer", resmsg2.MustError().Message)
}

func TestPassingHeader(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH1Handler(nil)
	server.Actor.ValidateSchema = true
	server.Actor.OnRequest("echoHeader", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		// echo the http reader X-Input back to client
		r := req.HttpRequest()
		resp := r.Header.Get("X-Input")
		return resp, nil
	})

	go ListenAndServe(rootCtx, "127.0.0.1:28050", server)
	time.Sleep(10 * time.Millisecond)

	client := NewH1Client(urlParse("http://127.0.0.1:28050"))

	client.SetExtraHeader(http.Header{"X-Input": []string{"Hello"}})
	// right request
	reqmsg := jsoff.NewRequestMessage(
		1, "echoHeader", nil)
	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.Equal("Hello", resmsg.MustResult())

}

func TestGatewayHandler(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewGatewayHandler(rootCtx, nil, false)
	server.Actor.On("echoAny", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return "ok", nil
		}
	})
	go ListenAndServe(rootCtx, "127.0.0.1:28450", server, serverTLS())
	time.Sleep(10 * time.Millisecond)

	// test http1 client
	client := NewH1Client(urlParse("https://127.0.0.1:28450"))
	client.SetClientTLSConfig(clientTLS())

	reqmsg := jsoff.NewRequestMessage(
		1, "echoAny", []interface{}{1991, 1992})
	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.Equal(json.Number("1991"), resmsg.MustResult())

	// test websocket
	client1 := NewWSClient(urlParse("wss://127.0.0.1:28450"))
	client1.SetClientTLSConfig(clientTLS())

	reqmsg1 := jsoff.NewRequestMessage(
		1001, "echoAny", []interface{}{8888})
	resmsg1, err1 := client1.Call(rootCtx, reqmsg1)
	assert.Nil(err1)
	assert.Equal(json.Number("8888"), resmsg1.MustResult())

	// test http2
	client2 := NewH2Client(urlParse("h2://127.0.0.1:28450"))
	client2.SetClientTLSConfig(clientTLS())

	reqmsg2 := jsoff.NewRequestMessage(
		2002, "echoAny", []interface{}{8886})
	resmsg2, err2 := client2.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.Equal(json.Number("8886"), resmsg2.MustResult())

}

func TestInsecureGatewayHandler(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewGatewayHandler(rootCtx, nil, true) // Insecure way
	server.Actor.On("echoAny", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return "ok", nil
		}
	})
	go ListenAndServe(rootCtx, "127.0.0.1:28453", server)
	time.Sleep(10 * time.Millisecond)

	_, errbadproto := NewClient("badproto")
	assert.Equal("url scheme not supported", errbadproto.Error())

	// test http1 client
	client, err := NewClient("http://127.0.0.1:28453")
	assert.Nil(err)
	// client is H1Client
	_, ok := client.(*H1Client)
	assert.True(ok)

	reqmsg := jsoff.NewRequestMessage(
		1, "echoAny", []interface{}{1991, 1992})
	resmsg, err := client.Call(rootCtx, reqmsg)
	assert.Nil(err)
	assert.Equal(json.Number("1991"), resmsg.MustResult())

	// test websocket
	//client1 := NewWSClient(urlParse("ws://127.0.0.1:28453"))
	client1, err := NewClient("ws://127.0.0.1:28453")
	assert.Nil(err)
	_, ok1 := client1.(*WSClient)
	assert.True(ok1)

	reqmsg1 := jsoff.NewRequestMessage(
		1001, "echoAny", []interface{}{8888})
	resmsg1, err1 := client1.Call(rootCtx, reqmsg1)
	assert.Nil(err1)
	assert.Equal(json.Number("8888"), resmsg1.MustResult())

	// test http2

	//client2 := NewH2Client(urlParse("h2c://127.0.0.1:28453"))
	client2, err := NewClient("h2c://127.0.0.1:28453")
	assert.Nil(err)
	_, ok2 := client2.(*H2Client)
	assert.True(ok2)

	reqmsg2 := jsoff.NewRequestMessage(
		2002, "echoAny", []interface{}{8886})

	resmsg2, err2 := client2.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.Equal(json.Number("8886"), resmsg2.MustResult())
}

func TestAuthorization(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, ok := AuthInfoFromContext(rootCtx)
	assert.False(ok)

	server := NewH1Handler(nil)
	server.Actor.OnRequest("greeting", func(req *RPCRequest, params []interface{}) (interface{}, error) {
		if authInfo, ok := AuthInfoFromContext(req.Context()); ok {
			return fmt.Sprintf("hello: %s", authInfo.Username), nil
		} else {
			return "anonymous", nil
		}

	})

	wrongauthcfg := &AuthConfig{
		Basic: []BasicAuthConfig{
			{},
		},
	}

	erremptyuserpass := wrongauthcfg.ValidateValues()
	assert.Equal("basic username or password are empty", erremptyuserpass.Error())

	wrongbearercfg := &AuthConfig{
		Bearer: []BearerAuthConfig{
			{},
		},
	}
	erremptybearer := wrongbearercfg.ValidateValues()
	assert.Equal("bearer token is empty", erremptybearer.Error())

	authcfg := &AuthConfig{
		Basic: []BasicAuthConfig{
			{
				Username: "monkey",
				Password: "banana",
			},
			{
				Username: "donkey",
				Password: "grass",
			},
		},

		Bearer: []BearerAuthConfig{
			{
				Token:    "bearbear",
				Username: "a_bear",
			},
		},
	}
	err0 := authcfg.ValidateValues()
	assert.Nil(err0)

	auth := NewAuthHandler(authcfg, server)
	go ListenAndServe(rootCtx, "127.0.0.1:28007", auth)
	time.Sleep(10 * time.Millisecond)

	client1 := NewH1Client(urlParse("http://127.0.0.1:28007"))
	reqmsg1 := jsoff.NewRequestMessage(jsoff.NewUuid(), "greeting", nil)

	_, err1 := client1.Call(rootCtx, reqmsg1)
	assert.NotNil(err1)
	var wrapped *WrappedResponse
	converted := errors.As(err1, &wrapped)
	assert.True(converted)
	assert.Equal(401, wrapped.Response.StatusCode)

	// client with correct user/pass
	client2 := NewH1Client(urlParse("http://donkey:grass@127.0.0.1:28007"))
	reqmsg2 := jsoff.NewRequestMessage(jsoff.NewUuid(), "greeting", nil)
	resmsg2, err2 := client2.Call(rootCtx, reqmsg2)
	assert.Nil(err2)
	assert.Equal("hello: donkey", resmsg2.MustResult())

	// client with correct bearer token
	client3 := NewH1Client(urlParse("http://127.0.0.1:28007"))
	client3.SetExtraHeader(http.Header{
		"Authorization": []string{"Bearer bearbear"},
	})
	reqmsg3 := jsoff.NewRequestMessage(jsoff.NewUuid(), "greeting", nil)
	resmsg3, err3 := client3.Call(rootCtx, reqmsg3)
	assert.Nil(err3)
	assert.Equal("hello: a_bear", resmsg3.MustResult())
}

func TestJwtAuthorization(t *testing.T) {
	assert := assert.New(t)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewH1Handler(nil)
	server.Actor.OnTypedRequest("greeting", func(req *RPCRequest) (*AuthInfo, error) {
		if authInfo, ok := AuthInfoFromContext(req.Context()); ok {
			return authInfo, nil
		} else {
			return nil, nil
		}

	})

	authcfg := &AuthConfig{
		Jwt: &JwtAuthConfig{Secret: "JwtIsUniversal"},
	}
	err0 := authcfg.ValidateValues()
	assert.Nil(err0)

	auth := NewAuthHandler(authcfg, server)
	go ListenAndServe(rootCtx, "127.0.0.1:28009", auth)
	time.Sleep(time.Millisecond * 100)

	// jwt auth
	claims := jwtClaims{
		Username: "jake",
		Settings: map[string]interface{}{"namespace": "jail"},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
			Issuer:    "jsoff.com",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenStr, err4 := token.SignedString([]byte("JwtIsUniversal"))
	assert.Nil(err4)

	client1 := NewH1Client(urlParse("http://127.0.0.1:28009"))
	client1.SetExtraHeader(http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", tokenStr)},
	})
	reqmsg1 := jsoff.NewRequestMessage(jsoff.NewUuid(), "greeting", nil)
	resmsg1, err1 := client1.Call(rootCtx, reqmsg1)
	assert.Nil(err1)

	var authinfo1 *AuthInfo
	errdecode := jsoff.DecodeInterface(resmsg1.MustResult(), &authinfo1)
	assert.Nil(errdecode)
	assert.NotNil(authinfo1)
	assert.Equal("jake", authinfo1.Username)
	ns, ok := authinfo1.Settings["namespace"]
	assert.True(ok)
	assert.Equal("jail", ns)
}

func TestActors(t *testing.T) {
	assert := assert.New(t)

	main_actor := NewActor()
	actor1 := NewActor()
	main_actor.AddChild(actor1)

	actor1.On("add2num", func(params []interface{}) (interface{}, error) {
		var tp struct {
			A int
			B int
		}
		err := jsoff.DecodeParams(params, &tp)
		if err != nil {
			return nil, err
		}
		return tp.A + tp.B, nil
	}, WithSchemaYaml(addSchemaYaml))

	main_actor.On("echo", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return nil, jsoff.ParamsError("no argument given")
		}
	})

	assert.True(main_actor.Has("echo"))
	assert.True(main_actor.Has("add2num"))

	assert.Equal([]string{"echo", "add2num"}, main_actor.MethodList())

	_, ok := main_actor.GetSchema("echo")
	assert.False(ok)

	_, ok = main_actor.GetSchema("add2num")
	assert.True(ok)

	actor1.Off("add2num")
	assert.False(main_actor.Has("add2num"))
}

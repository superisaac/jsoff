package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"github.com/superisaac/jsonz/http"
	"io"
	"os"
	"reflect"
	"time"
)

func main() {
	cliFlags := flag.NewFlagSet("jsonrpc-watch", flag.ExitOnError)
	pServerUrl := cliFlags.String("c", "", "jsonrpc server url, wss?, h2c? prefixed, can be in env JSONRPC_CONNECT, default is ws://127.0.0.1:9990")
	pRetry := cliFlags.Int("retry", 1, "retry times")
	var headerFlags jsonzhttp.HeaderFlags
	cliFlags.Var(&headerFlags, "header", "attached http headers")
	cliFlags.Parse(os.Args[1:])

	log.SetOutput(os.Stderr)

	// parse server url
	serverUrl := *pServerUrl
	if serverUrl == "" {
		serverUrl = os.Getenv("JSONRPC_CONNECT")
	}

	if serverUrl == "" {
		serverUrl = "ws://127.0.0.1:9990"
	}

	// parse http headers
	header, err := headerFlags.Parse()
	if err != nil {
		log.Fatalf("err parse header flags %s", err)
		os.Exit(1)
	}

	// parse method and params
	var method string
	var params []interface{}
	if cliFlags.NArg() >= 1 {
		args := cliFlags.Args()
		method = args[0]
		clParams := args[1:len(args)]

		p1, err := jsonz.GuessJsonArray(clParams)
		if err != nil {
			log.Fatalf("params error: %s", err)
			os.Exit(1)
		}
		params = p1
	}

	// jsonz client
	c, err := jsonzhttp.NewClient(serverUrl)
	if err != nil {
		log.Fatalf("fail to find jsonrpc client: %s", err)
		os.Exit(1)
	}
	c.SetExtraHeader(header)

	sc, ok := c.(jsonzhttp.Streamable)
	//if !c.IsStreaming() {
	if !ok {
		log.Panicf("streaming client required, but found %s", reflect.TypeOf(c))
		os.Exit(1)
	}

	sc.OnMessage(func(msg jsonz.Message) {
		repr, err := jsonz.EncodePretty(msg)
		if err != nil {
			//panic(err)
			log.Panicf("on message %s", err)
		}
		fmt.Println(repr)
	})

	watcher := &jsonrpcWatcher{
		retrylimit: *pRetry,
		sc:         sc,
		method:     method,
		params:     params,
	}

	watcher.run()

}

type jsonrpcWatcher struct {
	sc           jsonzhttp.Streamable
	method       string
	params       []interface{}
	retrylimit   int
	connectretry int
}

func (self *jsonrpcWatcher) run() {
	for {
		if err := self.connect(); err != nil {
			if errors.Is(err, jsonzhttp.TransportConnectFailed) ||
				errors.Is(err, jsonzhttp.TransportClosed) ||
				errors.Is(err, io.EOF) {

				self.connectretry++
				log.Infof("connect failed %d/%d times", self.connectretry, self.retrylimit)
				if self.connectretry >= self.retrylimit {
					break
				} else {
					time.Sleep(1 * time.Second)
					continue
				}
			} else {
				log.Errorf("watch error %s, %s", reflect.TypeOf(err), err)
				break
			}
		} else {
			break
		}
	}
}

func (self *jsonrpcWatcher) connect() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := self.sc.Connect(ctx); err != nil {
		return err
	}

	// reset connectretry after connected
	self.connectretry = 0

	if self.method != "" {
		reqId := jsonz.NewUuid()
		reqmsg := jsonz.NewRequestMessage(reqId, self.method, self.params)
		resmsg, err := self.sc.Call(ctx, reqmsg)
		if err != nil {
			log.Panicf("rpc error: %s", err)
			os.Exit(1)
		}
		repr, err := jsonz.EncodePretty(resmsg)
		if err != nil {
			log.Panicf("encode pretty error %s", err)
		}
		fmt.Println(repr)
	}
	// wait loop
	return self.sc.Wait()
}

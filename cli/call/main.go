package main

import (
	"context"
	"flag"
	"fmt"

	"os"

	"github.com/superisaac/jsoff"
	jsoffnet "github.com/superisaac/jsoff/net"
)

func main() {
	cliFlags := flag.NewFlagSet("jsonrpc-call", flag.ExitOnError)
	pServerUrl := cliFlags.String("c", "", "jsonrpc server url, https? or wss? prefixed, can be in env JSONRPC_CONNECT, default is http://127.0.0.1:9990")
	var headerFlags jsoffnet.HeaderFlags
	cliFlags.Var(&headerFlags, "header", "attached http headers")

	pDumpHeader := cliFlags.Bool("dumpheader", false, "dump response headers")

	cliFlags.Parse(os.Args[1:])

	if cliFlags.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "method params...\n")
		os.Exit(1)
	}

	// parse params
	args := cliFlags.Args()
	method := args[0]
	clParams := args[1:]

	params, err := jsoff.GuessJsonArray(clParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "params error: %s\n", err)
		os.Exit(1)
	}

	// parse server url
	serverUrl := *pServerUrl
	if serverUrl == "" {
		serverUrl = os.Getenv("JSONRPC_CONNECT")
	}

	if serverUrl == "" {
		serverUrl = "http://127.0.0.1:9990"
	}

	// parse http headers
	header, err := headerFlags.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "err parse header flags %s", err)
		os.Exit(1)
	}

	c, err := jsoffnet.NewClient(serverUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fail to find jsonrpc client: %s\n", err)
		os.Exit(1)
	}

	c.SetExtraHeader(header)

	reqId := jsoff.NewUuid()
	reqmsg := jsoff.NewRequestMessage(reqId, method, params)
	resmsg, err := c.Call(context.Background(), reqmsg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rpc error: %s\n", err)
		os.Exit(1)
	}

	repr, err := jsoff.EncodePretty(resmsg)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", repr)

	if *pDumpHeader {
		if responseMsg, ok := resmsg.(jsoff.ResponseMessage); ok && responseMsg.HasResponseHeader() {
			fmt.Println("Response header:")
			for header, values := range responseMsg.ResponseHeader() {
				for _, value := range values {
					fmt.Printf("  %s: %s\n", header, value)
				}
			}
		}
	}
}

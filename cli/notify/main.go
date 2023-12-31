package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/superisaac/jsoff"
	"github.com/superisaac/jsoff/net"
	"os"
)

func main() {
	cliFlags := flag.NewFlagSet("jsonrpc-notify", flag.ExitOnError)
	pServerUrl := cliFlags.String("c", "", "jsonrpc server url, https? or wss? prefixed, can be in env JSONRPC_CONNECT, default is http://127.0.0.1:9990")
	var headerFlags jsoffnet.HeaderFlags
	cliFlags.Var(&headerFlags, "header", "attached http headers")

	cliFlags.Parse(os.Args[1:])

	if cliFlags.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "method params...\n")
		os.Exit(1)
	}

	// parse params
	args := cliFlags.Args()
	method := args[0]
	clParams := args[1:len(args)]

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

	ntfmsg := jsoff.NewNotifyMessage(method, params)
	err = c.Send(context.Background(), ntfmsg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rpc error: %s\n", err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/superisaac/jsoff"
	"github.com/superisaac/jsoff/net"
	"net/http"
	"os"
	"sort"
	"time"
)

func main() {
	cliFlags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	pServerUrl := cliFlags.String("c", "", "jsonrpc server url, https? or wss? prefixed, can be in env JSONRPC_CONNECT, default is http://127.0.0.1:9990")
	pConcurrency := cliFlags.Uint("m", 10, "the number of concurrent clients")
	pNum := cliFlags.Uint("n", 10, "the number of calls per client")

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

	RunCallBenchmark(
		serverUrl,
		method, params,
		header,
		*pConcurrency, *pNum)
}

func toS(ns uint) float64 {
	return float64(ns) / float64(time.Second)
}

func RunCallBenchmark(serverUrl string, method string, params []interface{}, header http.Header, concurrency uint, num uint) {
	chResults := make(chan uint, concurrency*num)
	results := make([]uint, concurrency*num)
	var sum uint = 0

	for a := uint(0); a < concurrency; a++ {
		go callNTimes(chResults, serverUrl, method, params, header, num)
	}

	for i := uint(0); i < concurrency*num; i++ {
		usedTime := <-chResults
		sum += usedTime
		results[i] = usedTime
	}

	//sort.Uints(results)
	sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })

	avg := sum / uint(len(results))
	pos95 := int(0.95 * float64(len(results)))
	t95 := results[pos95]
	maxv := results[len(results)-1]
	minv := results[0]
	fmt.Printf("avg=%gs, min=%gs, p95=%gs, max=%gs\n", toS(avg), toS(minv), toS(t95), toS(maxv))
}

func callNTimes(chResults chan uint, serverUrl string, method string, params []interface{}, header http.Header, num uint) error {
	ctx := context.Background()
	c, err := jsoffnet.NewClient(serverUrl)
	if err != nil {
		panic(err)
	}
	c.SetExtraHeader(header)

	for i := uint(0); i < num; i++ {
		reqmsg := jsoff.NewRequestMessage(jsoff.NewUuid(), method, params)
		startTime := time.Now()
		_, err := c.Call(ctx, reqmsg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad results %d %s\n", i, err)
		}
		endTime := time.Now()
		chResults <- uint(endTime.Sub(startTime))
		//time.Sleep(10 * time.Millisecond)
	}
	return nil
}

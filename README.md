# jsoff

jsoff is a JSONRPC 2.0 library and a suite of client utilities in golang

# Install and build
```shell
% git checkout https://github.com/superisaac/jsoff
% cd jsoff
% make test # run unit test
...
% make build  # build cli tools
go build -gcflags=-G=3 -o bin/jsonrpc-call cli/call/jsonrpc_call.go
go build -gcflags=-G=3 -o bin/jsonrpc-notify cli/notify/jsonrpc_notify.go
go build -gcflags=-G=3 -o bin/jsonrpc-watch cli/watch/jsonrpc_watch.go
go build -gcflags=-G=3 -o bin/jsoff-example-fifo examples/fifo/main.go
```

# Examples
## Start a simple JSONRPC server
server side code
```go
import (
    // "github.com/superisaac/jsoff"
    "github.com/superisaac/jsoff/http"
)
// create a HTTP/1 handler, currently http1, http2 and websocket handlers are supported
server := jsoffhttp.NewH1Handler(nil)
// register an actor function
server.Actor.OnTyped("echo", func(req *jsoffhttp.RPCRequest, text string) (string, error) {
    return "echo " + text, nil
})
// serve the JSONRPC at port 8000
jsoffhttp.ListenAndServe(rootCtx, ":8000", server)
```
the server can be tested using client tools jsonrpc-call
```shell
% bin/jsonrpc-call -c http://127.0.0.1:8000 echo hi
{
   "jsonrpc": "2.0",
   "id": "1",
   "result": "echo hi"
}
```

## Initialize a JSONRPC request
```go
import (
    "context"
    "github.com/superisaac/jsoff"
    "github.com/superisaac/jsoff/http"
)

// create a jsonrpc client according to the server url
// the supported url schemes are: http, https, h2, h2c, ws and wss
client := jsoffhttp.NewClient("http://127.0.0.1:8000")

// create a request message with a random id field
reqmsg := jsoff.NewRequestMessage(jsoff.NewUuid(), "echo", []interface{}{"hi5"})
fmt.Printf("request message: %s\n", jsoff.MessageString(reqmsg))

resmsg, err := client.Call(context.Background(), reqmsg)
fmt.Printf("result message: %s\n", jsoff.MessageString(resmsg))
assert.True(resmsg.IsResultOrError())  // resmsg is a Result type message or an Error type message
assert.Equal("echo hi5", resmsg.MustResult())

// a notify message, notify message doesn't have id field and doesn't expect result
ntfmsg := jsoff.NewNotifyMessage("echo", []interface{}{"hi6"})
err := client.Send(context.Background(), ntfmsg)

```

## FIFO service
the FIFO service is an example to demonstrate how jsoff server and client works without writing and code. the server maintains an array in memory, you can push/pop/get items from it and list all items, you can even subscribe the item additions.

### Start server which listen at port 6000
```shell
% bin/jsoff-example-fifo
INFO[0000] Example fifo service starts at 127.0.0.1:6000
```

### Open another terminal and type a sequence of commands
```shell
# list the fifo and get an empty list
% bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_list
{
  "jsonrpc": "2.0",
  "id": "a85bfe31c94f4a5bb0fcb6539bbd6d66",
  "result": []
}

# push an item "hello"
 % bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_push hello
{
  "jsonrpc": "2.0",
  "id": "c460ebe3a9094249a043b6cddf3fa29f",
  "result": "ok"
}

# call list again, now that the item "hello" is pushed 
% bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_list
{
  "jsonrpc": "2.0",
  "id": "b3262cc3166e45e2bb5e58939d5e73bb",
  "result": [
    "hello"
  ]
}

# push another item 5(integer)
% bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_push 5
{
  "jsonrpc": "2.0",
  "id": "dc2bbac79e6841d78786e5ff5fc37c13",
  "result": "ok"
}

# now there are 2 items
% bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_list
{
  "jsonrpc": "2.0",
  "id": "29ca7b80ac504e9c9dd3513f3d4b966d",
  "result": [
    "hello",
    5
  ]
}

# get fifo[1], which is the second item of fifo
% bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_get 1
{
  "jsonrpc": "2.0",
  "id": "1c3fd5fe30034b72b115705263961c22",
  "result": 5
}

# pop an item out of the fifo
% bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_pop
{
  "jsonrpc": "2.0",
  "id": "df4bbe13c79c4fdc914ed8961ced9cf3",
  "result": "ok"
}

# the item 5 was removed
% bin/jsonrpc-call -c http://127.0.0.1:6000 fifo_list
{
  "jsonrpc": "2.0",
  "id": "f767a5275f544b9db2ba091cdebd9f5f",
  "result": [
    "hello"
  ]
}

```

## Open another terminal to subscribe item pushing
```shell
% bin/jsonrpc-watch -c h2c://127.0.0.1:6000 fifo_subscribe
{
  "jsonrpc": "2.0",
  "id": "abdc63d3873649a1a7a2b1bd49916e44",
  "result": "ok"
}
```
Note that the cli command is bin/jsonrpc-watch and the server url scheme is h2c:// which means the client can be in streaming mode, ws:// is also streaming schema but the http1 client doesn't support streaming.

now switch to the second terminal and push another item, the output turn out to be showed in the third terminal.
```shell
 % bin/jsonrpc-watch -c h2c://127.0.0.1:6000 fifo_subscribe
{
  "jsonrpc": "2.0",
  "id": "abdc63d3873649a1a7a2b1bd49916e44",
  "result": "ok"
}
{
  "jsonrpc": "2.0",
  "method": "fifo_subscription",
  "params": [
    "world"
  ]
}
```

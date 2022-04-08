# jlib

jlib is a JSONRPC 2.0 library and a suite of client utilities in golang

# Install and build
```shell
% git checkout https://github.com/superisaac/jlib
% cd jlib
% make test # run unit test
...
% make build  # build cli tools
go build -gcflags=-G=3 -o bin/jsonrpc-call cli/call/jsonrpc_call.go
go build -gcflags=-G=3 -o bin/jsonrpc-notify cli/notify/jsonrpc_notify.go
go build -gcflags=-G=3 -o bin/jsonrpc-watch cli/watch/jsonrpc_watch.go
go build -gcflags=-G=3 -o bin/jlib-example-fifo examples/fifo/main.go
```

# Examples
## Start a simple JSONRPC server
server side code
```go
import (
    // "github.com/superisaac/jlib"
    "github.com/superisaac/jlib/http"
)
// create a HTTP/1 handler, currently http1, http2 and websocket handlers are supported
server := jlibhttp.NewH1Handler(nil)
// register an actor function
server.Actor.OnTyped("echo", func(req *jlibhttp.RPCRequest, text string) (string, error) {
    return "echo " + text, nil
})
// serve the JSONRPC at port 8000
jlibhttp.ListenAndServe(rootCtx, ":8000", server)
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
    "github.com/superisaac/jlib"
)

// create a request message
reqmsg := jlib.NewRequestMessage(1, "echo", []interface{}{"hi5"})

// create a jsonrpc client according to the server url
// the supported url schemes are: http, https, h2, h2c, ws and wss
client := jlibhttp.NewClient("http://127.0.0.1:8000")

resmsg, err := client.Call(context.Background(), reqmsg)
assert.True(resmsg.IsResult())  // resmsg is a Result type message, it can also be an Error type message

```

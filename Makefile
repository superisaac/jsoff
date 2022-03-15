gofiles := $(shell find . -name '*.go')
goflag := -gcflags=-G=3

all: test

test:
	go test -v ./...

govet:
	go vet ./...
gofmt:
	go fmt ./...

build-cli: jsonrpc-call jsonrpc-notify jsonrpc-watch

jsonrpc-call: ${gofiles}
	go build $(goflag) -o $@ cli/call/jsonrpc_call.go

jsonrpc-notify: ${gofiles}
	go build $(goflag) -o $@ cli/notify/jsonrpc_notify.go

jsonrpc-watch: ${gofiles}
	go build $(goflag) -o $@ cli/watch/jsonrpc_watch.go

clean:
	rm -rf jsonrpc-cli build dist

.PHONY: test gofmt build-cli clean
.SECONDARY: $(buildarchdirs)

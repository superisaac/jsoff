gofiles := $(shell find . -name '*.go')
goflag := -gcflags=-G=3

all: test

test:
	go test -v ./...

govet:
	go vet ./...
gofmt:
	go fmt ./...

build: build-cli build-examples

build-cli: bin/jsonrpc-call bin/jsonrpc-notify bin/jsonrpc-watch

bin/jsonrpc-call: ${gofiles}
	go build $(goflag) -o $@ cli/call/jsonrpc_call.go

bin/jsonrpc-notify: ${gofiles}
	go build $(goflag) -o $@ cli/notify/jsonrpc_notify.go

bin/jsonrpc-watch: ${gofiles}
	go build $(goflag) -o $@ cli/watch/jsonrpc_watch.go

clean:
	rm -rf build dist bin/*

build-examples: bin/jsonz-example-fifo

bin/jsonz-example-fifo: ${gofiles}
	go build $(goflag) -o $@ examples/fifo/main.go

.PHONY: test gofmt build-cli clean
.SECONDARY: $(buildarchdirs)

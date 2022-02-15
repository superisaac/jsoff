gofiles := $(shell find . -name '*.go')
goflag := -gcflags=-G=3

all: test

test:
	go test -v ./...

govet:
	go vet ./...
gofmt:
	go fmt ./...

build-cli: jsonrpc-call jsonrpc-notify

jsonrpc-call: ${gofiles}
	go build $(goflag) -o $@ cli/call/jsonrpc_call.go

jsonrpc-notify: ${gofiles}
	go build $(goflag) -o $@ cli/notify/jsonrpc_notify.go

clean:
	rm -rf jsonrpc-cli build dist

compile-grpc: grpc/%.go

grpc/%.go: grpc/jsonz.proto
	protoc -I grpc/ --go_out=grpc --go-grpc_out=grpc $<

.PHONY: test gofmt build-cli compile-grpc clean
.SECONDARY: $(buildarchdirs)

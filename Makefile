gofiles := $(shell find . -name '*.go')
goflag := -gcflags=-G=3

all: test

test:
	go test -v ./...

govet:
	go vet ./...
gofmt:
	go fmt ./...

build-cli: jsonrpc-cli

jsonrpc-cli: ${gofiles}
	go build $(goflag) -o $@ cli/jsonrpc_client.go

clean:
	rm -rf jsonrpc-cli build dist

.PHONY: test gofmt build-cli clean
.SECONDARY: $(buildarchdirs)

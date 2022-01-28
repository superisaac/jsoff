gofiles := $(shell find . -name '*.go')
goflag := -gcflags=-G=3

all: test

test:
	go test -v github.com/superisaac/jsoz
	go test -v github.com/superisaac/jsoz/schema
	go test -v github.com/superisaac/jsoz/http

gofmt:
	go fmt *.go
	go fmt schema/*.go
	go fmt http/*.go
	go fmt cli/*.go

build-cli: jsonrpc-cli

jsonrpc-cli: ${gofiles}
	go build $(goflag) -o $@ cli/jsonrpc_client.go

clean:
	rm -rf jsonrpc-cli build dist

.PHONY: test gofmt build-cli clean
.SECONDARY: $(buildarchdirs)

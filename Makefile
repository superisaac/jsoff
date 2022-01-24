gofiles := $(shell find . -name '*.go')

all: test

test:
	go test -v github.com/superisaac/jsonrpc
	go test -v github.com/superisaac/jsonrpc/schema
	go test -v github.com/superisaac/jsonrpc/http

gofmt:
	go fmt *.go
	go fmt schema/*.go
	go fmt http/*.go

.PHONY: test gofmt
.SECONDARY: $(buildarchdirs)

gofiles := $(shell find . -name '*.go')

all: test

test:
	go test -v github.com/superisaac/jsonrpc
	go test -v github.com/superisaac/jsonrpc/schema

gofmt:
	go fmt *.go
	go fmt schema/*.go

.PHONY: test gofmt
.SECONDARY: $(buildarchdirs)

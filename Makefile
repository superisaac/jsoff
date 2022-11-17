gofiles := $(shell find . -name '*.go')
goflag :=

all: test

test:
	go test -v ./...

cover:
	go test -coverprofile=coverage.out  ./...
	@echo To view coverage graph use go tool cover -html=coverage.out

golint:
	go fmt ./...
	go vet ./...

build: build-cli build-examples

build-cli: bin/jsonrpc-call bin/jsonrpc-notify bin/jsonrpc-watch bin/jsonrpc-benchmark

bin/jsonrpc-call: ${gofiles}
	go build $(goflag) -o $@ cli/call/main.go

bin/jsonrpc-notify: ${gofiles}
	go build $(goflag) -o $@ cli/notify/main.go

bin/jsonrpc-watch: ${gofiles}
	go build $(goflag) -o $@ cli/watch/main.go

bin/jsonrpc-benchmark: ${gofiles}
	go build $(goflag) -o $@ cli/benchmark/main.go

clean:
	rm -rf build dist bin/*

build-examples: bin/jlib-example-fifo

bin/jlib-example-fifo: ${gofiles}
	go build $(goflag) -o $@ examples/fifo/main.go

.PHONY: test gofmt build-cli clean
.SECONDARY: $(buildarchdirs)

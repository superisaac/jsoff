gofiles := $(shell find . -name '*.go')

all: test

test:
	go test -v github.com/superisaac/jsoz
	go test -v github.com/superisaac/jsoz/schema
	go test -v github.com/superisaac/jsoz/http

gofmt:
	go fmt *.go
	go fmt schema/*.go
	go fmt http/*.go

.PHONY: test gofmt
.SECONDARY: $(buildarchdirs)

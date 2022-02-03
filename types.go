package jsonz

import (
	"github.com/bitly/go-simplejson"
	log "github.com/sirupsen/logrus"
)

type UID string

type RPCError struct {
	Code    int
	Message string
	Data    interface{}
}

const (
	MTRequest = 1
	MTNotify  = 2
	MTResult  = 3
	MTError   = 4
)

type Message interface {
	IsRequest() bool
	IsNotify() bool
	IsRequestOrNotify() bool
	IsResult() bool
	IsError() bool
	IsResultOrError() bool

	// TraceId
	SetTraceId(traceId string)
	TraceId() string

	// upvote
	GetJson() *simplejson.Json
	MustId() interface{}
	MustMethod() string
	MustParams() []interface{}
	MustResult() interface{}
	MustError() *RPCError

	ReplaceId(interface{}) Message

	Log() *log.Entry
}

type BaseMessage struct {
	messageType int
	raw         *simplejson.Json
	traceId     string
}

type RequestMessage struct {
	BaseMessage
	Id            interface{}
	Method        string
	Params        []interface{}
	paramsAreList bool

	// request specific fields

}

type NotifyMessage struct {
	BaseMessage
	Method        string
	Params        []interface{}
	paramsAreList bool
}

type ResultMessage struct {
	BaseMessage
	Id     interface{}
	Result interface{}
}

type ErrorMessage struct {
	BaseMessage
	Id    interface{}
	Error *RPCError
}

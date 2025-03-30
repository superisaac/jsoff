package jsoff

// implementations of message kinds

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// WithData create clone this RPCError object with data attached
func (e *RPCError) WithData(data any) *RPCError {
	return &RPCError{e.Code, e.Message, data}
}

// String representation of RPCError object
func (e *RPCError) Error() string {
	return fmt.Sprintf("code=%d, message=%s, data=%s", e.Code, e.Message, e.Data)
}

// Convert RPCError to ErrorMessage,  reqmsg is the original
// RequestMessage instance, the ErrorMessage will copy reqmsg's id
// property.
func (e RPCError) ToMessage(reqmsg *RequestMessage) *ErrorMessage {
	return RPCErrorMessage(reqmsg, e.Code, e.Message, e.Data)
}

// Convert RPCError to ErrorMessage, reqId and traceId can be used to
// compose the result error message
func (e RPCError) ToMessageFromId(reqId any, traceId string) *ErrorMessage {
	return RPCErrorMessageFromId(reqId, traceId, e.Code, e.Message, e.Data)
}

// Create a new instance of ErrMessageType
// additional is the information to help identify error details
func NewErrMsgType(additional string) *RPCError {
	r := fmt.Sprintf("wrong message type %s", additional)
	return &RPCError{ErrMessageType.Code, r, false}
}

// IsRequest() returns if the message is a RequestMessage
func (msg BaseMessage) IsRequest() bool {
	return msg.kind == MKRequest
}

// IsNotify() returns if the message is a NotifyMessage
func (msg BaseMessage) IsNotify() bool {
	return msg.kind == MKNotify
}

// IsResponse() returns if the message is a RequestMessage or
// NotifyMessage
func (msg BaseMessage) IsResponse() bool {
	return msg.IsRequest() || msg.IsNotify()
}

// IsResult() returns if the message is a ResultMessage
func (msg BaseMessage) IsResult() bool {
	return msg.kind == MKResult
}

// IsError() returns if the message is a ErrorMessage
func (msg BaseMessage) IsError() bool {
	return msg.kind == MKError
}

// IsResultOrError() returns if the message is a ResultMessage or
// ErrorMessage
func (msg BaseMessage) IsResultOrError() bool {
	return msg.IsResult() || msg.IsError()
}

// Message methods
func EncodePretty(msg Message) (string, error) {
	v := msg.Interface()
	bytes, err := json.MarshalIndent(v, "", "  ")
	//bytes, err := MessageJson(msg).EncodePretty()
	if err != nil {
		return "", errors.Wrap(err, "json.MarshalIndent")
	}
	return string(bytes), nil
}

func MessageString(msg Message) string {
	bytes, err := MessageBytes(msg)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

func MessageBytes(msg Message) ([]byte, error) {
	v := msg.Interface()
	return json.Marshal(v)
}

func MustMessageBytes(msg Message) []byte {
	bytes, err := MessageBytes(msg)
	if err != nil {
		panic(err)
	}
	return bytes
}

func MessageMap(msg Message) (map[string]any, error) {
	v := msg.Interface()
	m := map[string]any{}
	err := DecodeInterface(v, &m)
	if err != nil {
		return nil, err
	} else {
		if v, found := m["id"]; found {
			if vObj, ok := v.(map[string]any); ok {
				if msgId, ok := vObj["Value"]; ok {
					m["id"] = msgId
				}
			}
		}
		return m, nil
	}
}

func (msg *BaseMessage) SetTraceId(traceId string) {
	msg.traceId = traceId
}

func (msg BaseMessage) TraceId() string {
	return msg.traceId
}

// Log
func (msg RequestMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": msg.traceId,
		"msgtype": "request",
		"msgid":   msg.Id,
		"method":  msg.Method,
	})
}
func (msg NotifyMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": msg.traceId,
		"msgtype": "notify",
		"method":  msg.Method,
	})
}
func (msg ResultMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": msg.traceId,
		"msgtype": "result",
		"msgid":   msg.Id,
	})
}

func (msg ErrorMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": msg.traceId,
		"msgtype": "error",
		"msgid":   msg.Id,
	})
}

func (msg RequestMessage) ReplaceId(newId any) Message {
	return msg.Clone(newId)
}

func (msg NotifyMessage) ReplaceId(newId any) Message {
	panic(NewErrMsgType("ReplaceId"))
}

func (msg ResultMessage) ReplaceId(newId any) Message {
	resmsg := rawResultMessage(newId, msg.Result, msg.responseHeader)
	resmsg.SetTraceId(msg.TraceId())
	return resmsg
}

func (msg ErrorMessage) ReplaceId(newId any) Message {
	errmsg := rawErrorMessage(newId, msg.Error, msg.responseHeader)
	errmsg.SetTraceId(msg.TraceId())
	return errmsg
}

// Must methods

// MustId
func (msg RequestMessage) MustId() any {
	return msg.Id
}
func (msg NotifyMessage) MustId() any {
	panic(NewErrMsgType("MustId"))
}
func (msg ResultMessage) MustId() any {
	return msg.Id
}
func (msg ErrorMessage) MustId() any {
	return msg.Id
}

// MustMethod
func (msg RequestMessage) MustMethod() string {
	return msg.Method
}
func (msg NotifyMessage) MustMethod() string {
	return msg.Method
}
func (msg ResultMessage) MustMethod() string {
	panic(NewErrMsgType("MustMethod"))
}

func (msg ErrorMessage) MustMethod() string {
	panic(NewErrMsgType("MustMethod"))
}

// MustParams
func (msg RequestMessage) MustParams() []any {
	return msg.Params
}
func (msg NotifyMessage) MustParams() []any {
	return msg.Params
}
func (msg ResultMessage) MustParams() []any {
	panic(NewErrMsgType("MustParams"))
}
func (msg ErrorMessage) MustParams() []any {
	panic(NewErrMsgType("MustParams"))
}

// MustResult
func (msg RequestMessage) MustResult() any {
	panic(NewErrMsgType("MustResult"))
}
func (msg NotifyMessage) MustResult() any {
	panic(NewErrMsgType("MustResult"))
}
func (msg ResultMessage) MustResult() any {
	return msg.Result
}
func (msg ErrorMessage) MustResult() any {
	panic(NewErrMsgType("MustResult"))
}

// MustError
func (msg RequestMessage) MustError() *RPCError {
	panic(NewErrMsgType("MustError"))
}
func (msg NotifyMessage) MustError() *RPCError {
	panic(NewErrMsgType("MustError"))
}
func (msg ResultMessage) MustError() *RPCError {
	panic(NewErrMsgType("MustError"))
}
func (msg ErrorMessage) MustError() *RPCError {
	return msg.Error
}

// Interface
func (msg *RequestMessage) Interface() any {
	tmp := &templateRequest{
		Jsonrpc: "2.0",
		TraceId: msg.TraceId(),
		Method:  msg.Method,
		Id:      msgIdT{Value: msg.Id, isSet: true},
	}
	if msg.paramsAreList || len(msg.Params) == 0 {
		tmp.Params = msg.Params
	} else {
		tmp.Params = msg.Params[0]
	}
	return tmp
}

func (msg *NotifyMessage) Interface() any {
	tmp := &templateNotify{
		Jsonrpc: "2.0",
		TraceId: msg.TraceId(),
		Method:  msg.Method,
	}
	if msg.paramsAreList || len(msg.Params) == 0 {
		tmp.Params = msg.Params
	} else {
		tmp.Params = msg.Params[0]
	}
	return tmp
}

func (msg *ResultMessage) Interface() any {
	tmp := &templateResult{
		Jsonrpc: "2.0",
		TraceId: msg.TraceId(),
		Id:      msgIdT{Value: msg.Id, isSet: true},
		Result:  msg.Result,
	}
	return tmp
}

func (msg *ErrorMessage) Interface() any {
	tmp := &templateError{
		Jsonrpc: "2.0",
		TraceId: msg.TraceId(),
		Id:      msgIdT{Value: msg.Id, isSet: true},
		Error:   msg.Error,
	}
	return tmp
}

func NewRequestMessage(id any, method string, params any) *RequestMessage {
	if method == "" {
		panic(ErrEmptyMethod)
	}
	msg := &RequestMessage{}
	msg.kind = MKRequest
	msg.Id = id
	msg.Method = method
	msg.paramsAreList = true

	if params == nil {
		msg.Params = []any{}
	} else if arr, ok := params.([]any); ok {
		if arr == nil {
			arr = []any{}
		}
		msg.Params = arr
	} else {
		msg.Params = []any{params}
		msg.paramsAreList = false
	}
	return msg
}

func (msg RequestMessage) Clone(newId any) *RequestMessage {
	newReq := NewRequestMessage(newId, msg.Method, msg.Params)
	newReq.SetTraceId(msg.traceId)
	return newReq
}

func (msg RequestMessage) CacheKey(prefix string) string {
	paramBytes, err := json.Marshal(msg.Params)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s%s%s", prefix, msg.Method, string(paramBytes))
}

func NewNotifyMessage(method string, params any) *NotifyMessage {
	if method == "" {
		panic(ErrEmptyMethod)
	}

	msg := &NotifyMessage{}
	msg.kind = MKNotify
	msg.Method = method
	msg.paramsAreList = true

	if params == nil {
		msg.Params = []any{}
	} else if arr, ok := params.([]any); ok {
		if arr == nil {
			arr = []any{}
		}
		msg.Params = arr
	} else {
		msg.Params = []any{params}
		msg.paramsAreList = false
	}
	return msg
}

func rawResultMessage(id any, result any, responseHeader http.Header) *ResultMessage {
	msg := &ResultMessage{}
	msg.kind = MKResult
	msg.Id = id
	msg.Result = result
	msg.responseHeader = responseHeader
	return msg
}

// implements ResponseMessage
func (msg ResultMessage) HasResponseHeader() bool {
	return msg.responseHeader != nil
}

func (msg *ResultMessage) ResponseHeader() http.Header {
	if msg.responseHeader == nil {
		msg.responseHeader = http.Header{}
	}
	return msg.responseHeader
}

func (msg ErrorMessage) HasResponseHeader() bool {
	return msg.responseHeader != nil
}

func (msg *ErrorMessage) ResponseHeader() http.Header {
	if msg.responseHeader == nil {
		msg.responseHeader = http.Header{}
	}
	return msg.responseHeader
}

func NewResultMessage(reqmsg Message, result any) *ResultMessage {
	if reqmsg == nil {
		return rawResultMessage(nil, result, nil)
	} else {
		resmsg := rawResultMessage(reqmsg.MustId(), result, nil)
		resmsg.SetTraceId(reqmsg.TraceId())
		return resmsg
	}
}

func NewErrorMessage(reqmsg Message, errbody *RPCError) *ErrorMessage {
	if reqmsg == nil {
		return rawErrorMessage(nil, errbody, nil)
	}
	errmsg := rawErrorMessage(reqmsg.MustId(), errbody, nil)
	errmsg.SetTraceId(reqmsg.TraceId())
	return errmsg
}

func NewErrorMessageFromId(reqId any, traceId string, errbody *RPCError) *ErrorMessage {
	errmsg := rawErrorMessage(reqId, errbody, nil)
	errmsg.SetTraceId(traceId)
	return errmsg
}

func rawErrorMessage(id any, errbody *RPCError, responseHeader http.Header) *ErrorMessage {
	msg := &ErrorMessage{}
	msg.kind = MKError
	msg.Id = id
	msg.Error = errbody
	msg.responseHeader = responseHeader
	return msg
}

func RPCErrorMessage(reqmsg Message, code int, message string, data any) *ErrorMessage {
	errbody := &RPCError{code, message, data}
	return NewErrorMessage(reqmsg, errbody)
}

func RPCErrorMessageFromId(reqId any, traceId string, code int, message string, data any) *ErrorMessage {
	errbody := &RPCError{code, message, data}
	return NewErrorMessageFromId(reqId, traceId, errbody)
}

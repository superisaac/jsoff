package jsoff

// implementations of message kinds

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// WithData create clone this RPCError object with data attached
func (self *RPCError) WithData(data interface{}) *RPCError {
	return &RPCError{self.Code, self.Message, data}
}

// String representation of RPCError object
func (self *RPCError) Error() string {
	return fmt.Sprintf("code=%d, message=%s, data=%s", self.Code, self.Message, self.Data)
}

// Convert RPCError to ErrorMessage,  reqmsg is the original
// RequestMessage instance, the ErrorMessage will copy reqmsg's id
// property.
func (self RPCError) ToMessage(reqmsg *RequestMessage) *ErrorMessage {
	return RPCErrorMessage(reqmsg, self.Code, self.Message, self.Data)
}

// Convert RPCError to ErrorMessage, reqId and traceId can be used to
// compose the result error message
func (self RPCError) ToMessageFromId(reqId interface{}, traceId string) *ErrorMessage {
	return RPCErrorMessageFromId(reqId, traceId, self.Code, self.Message, self.Data)
}

// Create a new instance of ErrMessageType
// additional is the information to help identify error details
func NewErrMsgType(additional string) *RPCError {
	r := fmt.Sprintf("wrong message type %s", additional)
	return &RPCError{ErrMessageType.Code, r, false}
}

// IsRequest() returns if the message is a RequestMessage
func (self BaseMessage) IsRequest() bool {
	return self.kind == MKRequest
}

// IsNotify() returns if the message is a NotifyMessage
func (self BaseMessage) IsNotify() bool {
	return self.kind == MKNotify
}

// IsRequestOrNotify() returns if the message is a RequestMessage or
// NotifyMessage
func (self BaseMessage) IsRequestOrNotify() bool {
	return self.IsRequest() || self.IsNotify()
}

// IsResult() returns if the message is a ResultMessage
func (self BaseMessage) IsResult() bool {
	return self.kind == MKResult
}

// IsError() returns if the message is a ErrorMessage
func (self BaseMessage) IsError() bool {
	return self.kind == MKError
}

// IsResultOrError() returns if the message is a ResultMessage or
// ErrorMessage
func (self BaseMessage) IsResultOrError() bool {
	return self.IsResult() || self.IsError()
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

func MessageMap(msg Message) (map[string]interface{}, error) {
	v := msg.Interface()
	m := map[string]interface{}{}
	err := DecodeInterface(v, &m)
	if err != nil {
		return nil, err
	} else {
		return m, nil
	}
}

func (self *BaseMessage) SetTraceId(traceId string) {
	self.traceId = traceId
}

func (self BaseMessage) TraceId() string {
	return self.traceId
}

// Log
func (self RequestMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": self.traceId,
		"msgtype": "request",
		"msgid":   self.Id,
		"method":  self.Method,
	})
}
func (self NotifyMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": self.traceId,
		"msgtype": "notify",
		"method":  self.Method,
	})
}
func (self ResultMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": self.traceId,
		"msgtype": "result",
		"msgid":   self.Id,
	})
}

func (self ErrorMessage) Log() *log.Entry {
	return log.WithFields(log.Fields{
		"traceid": self.traceId,
		"msgtype": "error",
		"msgid":   self.Id,
	})
}

func (self RequestMessage) ReplaceId(newId interface{}) Message {
	return self.Clone(newId)
}

func (self NotifyMessage) ReplaceId(newId interface{}) Message {
	panic(NewErrMsgType("ReplaceId"))
}

func (self ResultMessage) ReplaceId(newId interface{}) Message {
	resmsg := rawResultMessage(newId, self.Result)
	resmsg.SetTraceId(self.TraceId())
	return resmsg
}

func (self ErrorMessage) ReplaceId(newId interface{}) Message {
	errmsg := rawErrorMessage(newId, self.Error)
	errmsg.SetTraceId(self.TraceId())
	return errmsg
}

// Must methods

// MustId
func (self RequestMessage) MustId() interface{} {
	return self.Id
}
func (self NotifyMessage) MustId() interface{} {
	panic(NewErrMsgType("MustId"))
}
func (self ResultMessage) MustId() interface{} {
	return self.Id
}
func (self ErrorMessage) MustId() interface{} {
	return self.Id
}

// MustMethod
func (self RequestMessage) MustMethod() string {
	return self.Method
}
func (self NotifyMessage) MustMethod() string {
	return self.Method
}
func (self ResultMessage) MustMethod() string {
	panic(NewErrMsgType("MustMethod"))
}

func (self ErrorMessage) MustMethod() string {
	panic(NewErrMsgType("MustMethod"))
}

// MustParams
func (self RequestMessage) MustParams() []interface{} {
	return self.Params
}
func (self NotifyMessage) MustParams() []interface{} {
	return self.Params
}
func (self ResultMessage) MustParams() []interface{} {
	panic(NewErrMsgType("MustParams"))
}
func (self ErrorMessage) MustParams() []interface{} {
	panic(NewErrMsgType("MustParams"))
}

// MustResult
func (self RequestMessage) MustResult() interface{} {
	panic(NewErrMsgType("MustResult"))
}
func (self NotifyMessage) MustResult() interface{} {
	panic(NewErrMsgType("MustResult"))
}
func (self ResultMessage) MustResult() interface{} {
	return self.Result
}
func (self ErrorMessage) MustResult() interface{} {
	panic(NewErrMsgType("MustResult"))
}

// MustError
func (self RequestMessage) MustError() *RPCError {
	panic(NewErrMsgType("MustError"))
}
func (self NotifyMessage) MustError() *RPCError {
	panic(NewErrMsgType("MustError"))
}
func (self ResultMessage) MustError() *RPCError {
	panic(NewErrMsgType("MustError"))
}
func (self ErrorMessage) MustError() *RPCError {
	return self.Error
}

// Interface
func (self *RequestMessage) Interface() interface{} {
	tmp := &templateRequest{
		Jsonrpc: "2.0",
		TraceId: self.TraceId(),
		Method:  self.Method,
		Id:      self.Id,
	}
	if self.paramsAreList || len(self.Params) == 0 {
		tmp.Params = self.Params
	} else {
		tmp.Params = self.Params[0]
	}
	return tmp
}

func (self *NotifyMessage) Interface() interface{} {
	tmp := &templateNotify{
		Jsonrpc: "2.0",
		TraceId: self.TraceId(),
		Method:  self.Method,
	}
	if self.paramsAreList || len(self.Params) == 0 {
		tmp.Params = self.Params
	} else {
		tmp.Params = self.Params[0]
	}
	return tmp
}

func (self *ResultMessage) Interface() interface{} {
	tmp := &templateResult{
		Jsonrpc: "2.0",
		TraceId: self.TraceId(),
		Id:      self.Id,
		Result:  self.Result,
	}
	return tmp
}

func (self *ErrorMessage) Interface() interface{} {
	tmp := &templateError{
		Jsonrpc: "2.0",
		TraceId: self.TraceId(),
		Id:      self.Id,
		Error:   self.Error,
	}
	return tmp
}

func NewRequestMessage(id interface{}, method string, params interface{}) *RequestMessage {
	if id == nil {
		panic(ErrNilId)
	}
	if method == "" {
		panic(ErrEmptyMethod)
	}

	msg := &RequestMessage{}
	msg.kind = MKRequest
	msg.Id = id
	msg.Method = method
	msg.paramsAreList = true

	if params == nil {
		msg.Params = []interface{}{}
	} else if arr, ok := params.([]interface{}); ok {
		if arr == nil {
			arr = []interface{}{}
		}
		msg.Params = arr
	} else {
		msg.Params = []interface{}{params}
		msg.paramsAreList = false
	}
	return msg
}

func (self RequestMessage) Clone(newId interface{}) *RequestMessage {
	newReq := NewRequestMessage(newId, self.Method, self.Params)
	newReq.SetTraceId(self.traceId)
	return newReq
}

func (self RequestMessage) CacheKey(prefix string) string {
	paramBytes, err := json.Marshal(self.Params)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s%s%s", prefix, self.Method, string(paramBytes))
}

func NewNotifyMessage(method string, params interface{}) *NotifyMessage {
	if method == "" {
		panic(ErrEmptyMethod)
	}

	msg := &NotifyMessage{}
	msg.kind = MKNotify
	msg.Method = method
	msg.paramsAreList = true

	if params == nil {
		msg.Params = []interface{}{}
	} else if arr, ok := params.([]interface{}); ok {
		if arr == nil {
			arr = []interface{}{}
		}
		msg.Params = arr
	} else {
		msg.Params = []interface{}{params}
		msg.paramsAreList = false
	}
	return msg
}

func rawResultMessage(id interface{}, result interface{}) *ResultMessage {
	msg := &ResultMessage{}
	msg.kind = MKResult
	msg.Id = id
	msg.Result = result
	return msg
}

func NewResultMessage(reqmsg Message, result interface{}) *ResultMessage {
	resmsg := rawResultMessage(reqmsg.MustId(), result)
	resmsg.SetTraceId(reqmsg.TraceId())
	return resmsg
}

func NewErrorMessage(reqmsg Message, errbody *RPCError) *ErrorMessage {
	errmsg := rawErrorMessage(reqmsg.MustId(), errbody)
	errmsg.SetTraceId(reqmsg.TraceId())
	return errmsg
}

func NewErrorMessageFromId(reqId interface{}, traceId string, errbody *RPCError) *ErrorMessage {
	errmsg := rawErrorMessage(reqId, errbody)
	errmsg.SetTraceId(traceId)
	return errmsg
}

func rawErrorMessage(id interface{}, errbody *RPCError) *ErrorMessage {
	msg := &ErrorMessage{}
	msg.kind = MKError
	msg.Id = id
	msg.Error = errbody
	return msg
}

func RPCErrorMessage(reqmsg Message, code int, message string, data interface{}) *ErrorMessage {
	errbody := &RPCError{code, message, data}
	return NewErrorMessage(reqmsg, errbody)
}

func RPCErrorMessageFromId(reqId interface{}, traceId string, code int, message string, data interface{}) *ErrorMessage {
	errbody := &RPCError{code, message, data}
	return NewErrorMessageFromId(reqId, traceId, errbody)
}

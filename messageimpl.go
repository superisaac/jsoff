package jsonrpc

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// WithData create clone this RPCError object with data attached
func (self *RPCError) WithData(data interface{}) *RPCError {
	return &RPCError{self.Code, self.Message, data}
}

// String Representation of RPCError object
func (self *RPCError) Error() string {
	return fmt.Sprintf("code=%d, message=%s, data=%s", self.Code, self.Message, self.Data)
}

// Convert RPCError to ErrorMessage
// reqmsg is the original RequestMessage instance, the ErrorMessage
// will copy reqmsg's id property
func (self RPCError) ToMessage(reqmsg *RequestMessage) *ErrorMessage {
	return RPCErrorMessage(reqmsg, self.Code, self.Message, self.Data)
}

// Convert RPCError to ErrorMessage
// reqmsg is the original RequestMessage instance, the ErrorMessage
// will copy reqmsg's id property
func (self RPCError) ToMessageFromId(reqId interface{}, traceId string) *ErrorMessage {
	return RPCErrorMessageFromId(reqId, traceId, self.Code, self.Message, self.Data)
}

// Generate json represent of ErrorMessage.body
// refer to https://www.jsonrpc.org/specification#error_object
func (self RPCError) ToJson() *simplejson.Json {
	json := simplejson.New()
	json.Set("code", self.Code)
	json.Set("message", self.Message)
	if self.Data != nil {
		json.Set("data", self.Data)
	}
	return json
}

// Create a new instance of ErrMessageType
// additional is the information to help identify error details
func NewErrMsgType(additional string) *RPCError {
	r := fmt.Sprintf("wrong message type %s", additional)
	return &RPCError{ErrMessageType.Code, r, false}
}

// Set raw Json object to skip generating the same raw
// a little tip
func (self *BaseMessage) SetRaw(raw *simplejson.Json) {
	self.raw = raw
}

// IsRequest() returns if the message is a RequestMessage
func (self BaseMessage) IsRequest() bool {
	return self.messageType == MTRequest
}

// IsNotify() returns if the message is a NotifyMessage
func (self BaseMessage) IsNotify() bool {
	return self.messageType == MTNotify
}

// IsRequestOrNotify() returns if the message is a RequestMessage or
// NotifyMessage
func (self BaseMessage) IsRequestOrNotify() bool {
	return self.IsRequest() || self.IsNotify()
}

// IsResult() returns if the message is a ResultMessage
func (self BaseMessage) IsResult() bool {
	return self.messageType == MTResult
}

// IsError() returns if the message is a ErrorMessage
func (self BaseMessage) IsError() bool {
	return self.messageType == MTError
}

// IsResultOrError() returns if the message is a ResultMessage or
// ErrorMessage
func (self BaseMessage) IsResultOrError() bool {
	return self.IsResult() || self.IsError()
}

// IMessage methods
func EncodePretty(msg IMessage) (string, error) {
	bytes, err := MessageJson(msg).EncodePretty()
	if err != nil {
		return "", errors.Wrap(err, "simplejson.Json.EncodePretty()")
	}
	return string(bytes), nil
}

// A Message object to Json object for further trans
func MessageJson(msg IMessage) *simplejson.Json {
	jsonObj := msg.GetJson()
	if traceId := msg.TraceId(); traceId != "" {
		jsonObj.Set("traceid", traceId)
	}
	return jsonObj
}

func MessageInterface(msg IMessage) interface{} {
	return MessageJson(msg).Interface()
}

func MessageString(msg IMessage) string {
	bytes, err := MessageBytes(msg)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

func MessageBytes(msg IMessage) ([]byte, error) {
	return MessageJson(msg).MarshalJSON()
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

// Get Json
func (self *RequestMessage) GetJson() *simplejson.Json {
	if self.raw == nil {
		self.raw = simplejson.New()
		self.raw.Set("jsonrpc", "2.0")
		self.raw.Set("id", self.Id)
		self.raw.Set("method", self.Method)
		if self.paramsAreList || len(self.Params) == 0 {
			self.raw.Set("params", self.Params)
		} else {
			self.raw.Set("params", self.Params[0])
		}
	}
	return self.raw
}

func (self *NotifyMessage) GetJson() *simplejson.Json {
	if self.raw == nil {
		self.raw = simplejson.New()
		self.raw.Set("jsonrpc", "2.0")
		self.raw.Set("method", self.Method)
		if self.paramsAreList || len(self.Params) == 0 {
			self.raw.Set("params", self.Params)
		} else {
			self.raw.Set("params", self.Params[0])
		}
	}
	return self.raw
}

func (self *ResultMessage) GetJson() *simplejson.Json {
	if self.raw == nil {
		self.raw = simplejson.New()
		self.raw.Set("jsonrpc", "2.0")
		self.raw.Set("id", self.Id)
		self.raw.Set("result", self.Result)
	}
	return self.raw
}

func (self *ErrorMessage) GetJson() *simplejson.Json {
	if self.raw == nil {
		self.raw = simplejson.New()
		self.raw.Set("jsonrpc", "2.0")
		self.raw.Set("id", self.Id)
		self.raw.Set("error", self.Error.ToJson())
	}
	return self.raw
}

func NewRequestMessage(id interface{}, method string, params []interface{}) *RequestMessage {
	if id == nil {
		panic(ErrNilId)
	}
	if method == "" {
		panic(ErrEmptyMethod)
	}

	if params == nil {
		params = [](interface{}){}
	}

	msg := &RequestMessage{}
	msg.messageType = MTRequest
	msg.Id = id
	msg.Method = method
	msg.Params = params
	msg.paramsAreList = true
	return msg
}

func (self RequestMessage) Clone(newId interface{}) *RequestMessage {
	newReq := NewRequestMessage(newId, self.Method, self.Params)
	newReq.SetTraceId(self.traceId)
	return newReq
}

func NewNotifyMessage(method string, params []interface{}) *NotifyMessage {
	if method == "" {
		panic(ErrEmptyMethod)
	}

	if params == nil {
		params = [](interface{}){}
	}

	msg := &NotifyMessage{}
	msg.messageType = MTNotify
	msg.Method = method
	msg.Params = params
	msg.paramsAreList = true
	return msg
}

func rawResultMessage(id interface{}, result interface{}) *ResultMessage {
	msg := &ResultMessage{}
	msg.messageType = MTResult
	msg.Id = id
	msg.Result = result
	return msg
}

func NewResultMessage(reqmsg IMessage, result interface{}) *ResultMessage {
	resmsg := rawResultMessage(reqmsg.MustId(), result)
	resmsg.SetTraceId(reqmsg.TraceId())
	return resmsg
}

func NewErrorMessage(reqmsg IMessage, errbody *RPCError) *ErrorMessage {
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
	msg.messageType = MTError
	msg.Id = id
	msg.Error = errbody
	return msg
}

func RPCErrorMessage(reqmsg IMessage, code int, message string, data interface{}) *ErrorMessage {
	errbody := &RPCError{code, message, data}
	return NewErrorMessage(reqmsg, errbody)
}

func RPCErrorMessageFromId(reqId interface{}, traceId string, code int, message string, data interface{}) *ErrorMessage {
	errbody := &RPCError{code, message, data}
	return NewErrorMessageFromId(reqId, traceId, errbody)
}

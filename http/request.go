package jsoffhttp

import (
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
	"github.com/superisaac/jsoff/schema"
	"net/http"
)

const (
	TransportHTTP      = "http"
	TransportWebsocket = "websocket"
	TransportHTTP2     = "http2"
	TransportTCP       = "tcp"
)

type RPCSession interface {
	Send(msg jsoff.Message)
	SessionID() string
}

// http rpc quest structure
type RPCRequest struct {
	context       context.Context
	msg           jsoff.Message
	transportType string
	r             *http.Request
	data          interface{} // arbitrary data
	session       RPCSession
}

func NewRPCRequest(ctx context.Context, msg jsoff.Message, transportType string) *RPCRequest {
	return &RPCRequest{
		context:       ctx,
		msg:           msg,
		transportType: transportType,
	}
}

func (self *RPCRequest) WithSession(session RPCSession) *RPCRequest {
	self.session = session
	return self
}

func (self *RPCRequest) WithHTTPRequest(r *http.Request) *RPCRequest {
	self.r = r
	return self
}

func (self RPCRequest) Context() context.Context {
	return self.context
}

func (self RPCRequest) Msg() jsoff.Message {
	return self.msg
}

func (self RPCRequest) Session() RPCSession {
	return self.session
}

func (self RPCRequest) HttpRequest() *http.Request {
	if self.r == nil {
		panic("Http Request is nil")
	}
	return self.r
}

func (self RPCRequest) Data() interface{} {
	return self.data
}

func (self RPCRequest) Log() *log.Entry {
	remoteAddr := ""
	if self.r != nil {
		remoteAddr = self.r.RemoteAddr
	}
	return self.msg.Log().WithFields(log.Fields{
		"ttype":      self.transportType,
		"remoteAddr": remoteAddr,
	})
}

// handler func
type RequestCallback func(req *RPCRequest, params []interface{}) (interface{}, error)

type ContextedMsgCallback func(ctx context.Context, params []interface{}) (interface{}, error)
type MsgCallback func(params []interface{}) (interface{}, error)
type MissingCallback func(req *RPCRequest) (interface{}, error)

// type CloseCallback func(r *http.Request, session RPCSession)
type CloseCallback func(session RPCSession)

// With method handler
type MethodHandler struct {
	callback RequestCallback
	schema   jsoffschema.Schema
}

type HandlerSetter func(h *MethodHandler)

func WithSchema(s jsoffschema.Schema) HandlerSetter {
	return func(h *MethodHandler) {
		h.schema = s
	}
}

func WithSchemaYaml(yamlSchema string) HandlerSetter {
	builder := jsoffschema.NewSchemaBuilder()
	s, err := builder.BuildYamlBytes([]byte(yamlSchema))
	if err != nil {
		panic(err)
	}
	return WithSchema(s)
}

func WithSchemaJson(jsonSchema string) HandlerSetter {
	builder := jsoffschema.NewSchemaBuilder()
	s, err := builder.BuildBytes([]byte(jsonSchema))
	if err != nil {
		panic(err)
	}
	return WithSchema(s)
}

type Actor struct {
	ValidateSchema   bool
	RecoverFromPanic bool
	methodHandlers   map[string]*MethodHandler
	missingHandler   MissingCallback
	closeHandler     CloseCallback
	children         []*Actor
}

func NewActor() *Actor {
	return &Actor{
		ValidateSchema:   true,
		RecoverFromPanic: true,

		methodHandlers: make(map[string]*MethodHandler),
		children:       make([]*Actor, 0),
	}
}

func (self *Actor) AddChild(child *Actor) {
	self.children = append(self.children, child)
}

// register a method handler
func (self *Actor) On(method string, callback MsgCallback, setters ...HandlerSetter) {

	reqcb := func(req *RPCRequest, params []interface{}) (interface{}, error) {
		return callback(params)
	}
	err := self.OnRequest(method, reqcb, setters...)
	if err != nil {
		panic(err)
	}
}

func (self *Actor) OnContext(method string, callback ContextedMsgCallback, setters ...HandlerSetter) {

	reqcb := func(req *RPCRequest, params []interface{}) (interface{}, error) {
		return callback(req.Context(), params)
	}
	err := self.OnRequest(method, reqcb, setters...)
	if err != nil {
		panic(err)
	}
}

func (self *Actor) OnRequest(method string, callback RequestCallback, setters ...HandlerSetter) error {
	if _, exist := self.methodHandlers[method]; exist {
		return errors.New("handler already exist!")
	}
	h := &MethodHandler{
		callback: callback,
	}

	for _, setter := range setters {
		setter(h)
	}
	self.methodHandlers[method] = h
	return nil
}

// register a typed method handler
func (self *Actor) OnTyped(method string, typedHandler interface{}, setters ...HandlerSetter) {
	handler, err := wrapTyped(typedHandler, nil)
	if err != nil {
		panic(err)
	}
	err = self.OnRequest(method, handler, setters...)
	if err != nil {
		panic(err)
	}
}

func (self *Actor) OnTypedRequest(method string, typedHandler interface{}, setters ...HandlerSetter) error {
	//firstArg := reflect.TypeOf(&RPCRequest{})
	handler, err := wrapTyped(typedHandler, &ReqSpec{})
	if err != nil {
		return err
	}
	return self.OnRequest(method, handler, setters...)
}

func (self *Actor) OnTypedContext(method string, typedHandler interface{}, setters ...HandlerSetter) error {
	//firstArgSpec := reflect.TypeOf((*context.Context)(nil)).Elem()
	handler, err := wrapTyped(typedHandler, &ContextSpec{})
	if err != nil {
		return err
	}
	return self.OnRequest(method, handler, setters...)
}

// Off unregister the method from handlers
func (self *Actor) Off(method string) {
	delete(self.methodHandlers, method)
}

// register a callback called when no hander to handle a request
// message or non-request message met
func (self *Actor) OnMissing(handler MissingCallback) error {
	if self.missingHandler != nil {
		return errors.New("missing handler already exist!")
	}
	self.missingHandler = handler
	return nil
}

// OnClose handler is called when the stream beneath the actor is closed
func (self *Actor) OnClose(handler CloseCallback) error {
	if self.closeHandler != nil {
		return errors.New("close handler already exist!")
	}
	self.closeHandler = handler
	return nil
}

// call the close handler if possible
func (self *Actor) HandleClose(session RPCSession) {
	// each child have to be called
	for _, child := range self.children {
		child.HandleClose(session)
	}

	if self.closeHandler != nil {
		self.closeHandler(session)
	}
}

// returns there is a handler for a method
func (self Actor) Has(method string) bool {
	if _, exist := self.methodHandlers[method]; exist {
		return true
	}

	for _, child := range self.children {
		if child.Has(method) {
			return true
		}
	}
	return false
}

func (self Actor) MethodList() []string {
	methods := []string{}
	for mname, _ := range self.methodHandlers {
		methods = append(methods, mname)
	}
	for _, child := range self.children {
		childMethods := child.MethodList()
		methods = append(methods, childMethods...)
	}
	return methods
}

// get the schema of a method
func (self Actor) GetSchema(method string) (jsoffschema.Schema, bool) {
	if h, ok := self.getHandler(method); ok && h.schema != nil {
		return h.schema, true
	}
	for _, child := range self.children {
		if s, ok := child.GetSchema(method); ok {
			return s, ok
		}
	}
	return nil, false
}

// get the handler of a method
func (self *Actor) getHandler(method string) (*MethodHandler, bool) {
	if h, ok := self.methodHandlers[method]; ok {
		return h, true
	} else {
		return nil, false
	}
}

// give the actor a request message
func (self *Actor) Feed(req *RPCRequest) (jsoff.Message, error) {
	msg := req.Msg()
	if !msg.IsRequestOrNotify() {
		if self.missingHandler != nil {
			res, err := self.missingHandler(req)
			return self.wrapResult(res, err, msg)
		} else {
			req.Log().Info("no handler to handle this message")
			return nil, nil
		}
	}

	// TODO: recover from panic
	if handler, found := self.getHandler(msg.MustMethod()); found {
		params := msg.MustParams()
		if handler.schema != nil && self.ValidateSchema {
			// validate the request
			validator := jsoffschema.NewSchemaValidator()
			m, err := jsoff.MessageMap(msg)
			if err != nil {
				return nil, err
			}
			errPos := validator.Validate(handler.schema, m)
			if errPos != nil {
				if reqmsg, ok := msg.(*jsoff.RequestMessage); ok {
					return errPos.ToMessage(reqmsg), nil
				}
				return nil, errPos
			}
		}
		return self.recoverCallHandler(handler, req, params)
	} else {
		for _, child := range self.children {
			if child.Has(msg.MustMethod()) {
				return child.Feed(req)
			}
		}
		if self.missingHandler != nil {
			return self.recoverCallMissingHandler(req)
		} else {
			if msg.IsRequest() {
				return jsoff.ErrMethodNotFound.ToMessageFromId(
					msg.MustId(), msg.TraceId()), nil
			}
		}
	}
	return nil, nil
}

func (self Actor) recoverCallHandler(handler *MethodHandler, req *RPCRequest, params []interface{}) (resmsg0 jsoff.Message, err0 error) {
	if self.RecoverFromPanic {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					resmsg0, err0 = self.wrapResult(nil, err, req.Msg())
				} else {
					panic(r)
				}
			}
		}()
	}
	res, err := handler.callback(req, params)
	return self.wrapResult(res, err, req.Msg())
}

func (self Actor) recoverCallMissingHandler(req *RPCRequest) (resmsg0 jsoff.Message, err0 error) {
	if self.RecoverFromPanic {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					resmsg0, err0 = self.wrapResult(nil, err, req.Msg())
				} else {
					// rethrown the panic result
					panic(r)
				}
			}
		}()
	}
	res, err := self.missingHandler(req)
	return self.wrapResult(res, err, req.Msg())
}

func (self Actor) wrapResult(res interface{}, err error, msg jsoff.Message) (jsoff.Message, error) {
	if !msg.IsRequest() {
		if err != nil {
			msg.Log().Errorf("wrapResult(), error handleing res, %#v", err)
		}
		return nil, err
	}

	reqmsg, ok := msg.(*jsoff.RequestMessage)
	if !ok {
		msg.Log().Panicf("convert to request message failed")
		return nil, err
	}

	if err != nil {
		var rpcErr *jsoff.RPCError
		var wrapErr *WrappedResponse
		if errors.As(err, &rpcErr) {
			return rpcErr.ToMessage(reqmsg), nil
		} else if errors.As(err, &wrapErr) {
			return nil, wrapErr
		} else {
			msg.Log().Errorf("wrapResult(), error handling err %#v", err)
			return jsoff.ErrInternalError.ToMessage(reqmsg), nil
		}
	} else if resmsg1, ok := res.(jsoff.Message); ok {
		// normal response
		return resmsg1, nil
	} else {
		return jsoff.NewResultMessage(reqmsg, res), nil
	}
}

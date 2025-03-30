package jsoffnet

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
	TransportVsock     = "vsock"
)

type RPCSession interface {
	Context() context.Context
	Send(msg jsoff.Message)
	SessionID() string
}

// http rpc quest structure
type RPCRequest struct {
	context       context.Context
	msg           jsoff.Message
	transportType string
	r             *http.Request
	data          any // arbitrary data
	session       RPCSession
}

func NewRPCRequest(ctx context.Context, msg jsoff.Message, transportType string) *RPCRequest {
	return &RPCRequest{
		context:       ctx,
		msg:           msg,
		transportType: transportType,
	}
}

func (req *RPCRequest) WithSession(session RPCSession) *RPCRequest {
	req.session = session
	return req
}

func (req *RPCRequest) WithHTTPRequest(r *http.Request) *RPCRequest {
	req.r = r
	return req
}

func (req RPCRequest) Context() context.Context {
	return req.context
}

func (req RPCRequest) Msg() jsoff.Message {
	return req.msg
}

func (req RPCRequest) Session() RPCSession {
	return req.session
}

func (req RPCRequest) HttpRequest() *http.Request {
	if req.r == nil {
		panic("Http Request is nil")
	}
	return req.r
}

func (req RPCRequest) Data() any {
	return req.data
}

func (req RPCRequest) Log() *log.Entry {
	remoteAddr := ""
	if req.r != nil {
		remoteAddr = req.r.RemoteAddr
	}
	return req.msg.Log().WithFields(log.Fields{
		"ttype":      req.transportType,
		"remoteAddr": remoteAddr,
	})
}

// handler func
type RequestCallback func(req *RPCRequest, params []any) (any, error)

type ContextedMsgCallback func(ctx context.Context, params []any) (any, error)
type MsgCallback func(params []any) (any, error)
type MissingCallback func(req *RPCRequest) (any, error)

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
	a := &Actor{
		ValidateSchema:   true,
		RecoverFromPanic: true,

		methodHandlers: make(map[string]*MethodHandler),
		children:       make([]*Actor, 0),
	}
	return a
}

func (a *Actor) AddChild(child *Actor) {
	a.children = append(a.children, child)
}

// register a method handler
func (a *Actor) On(method string, callback MsgCallback, setters ...HandlerSetter) {

	reqcb := func(req *RPCRequest, params []any) (any, error) {
		return callback(params)
	}
	err := a.OnRequest(method, reqcb, setters...)
	if err != nil {
		panic(err)
	}
}

func (a *Actor) OnContext(method string, callback ContextedMsgCallback, setters ...HandlerSetter) {

	reqcb := func(req *RPCRequest, params []any) (any, error) {
		return callback(req.Context(), params)
	}
	err := a.OnRequest(method, reqcb, setters...)
	if err != nil {
		panic(err)
	}
}

func (a *Actor) OnRequest(method string, callback RequestCallback, setters ...HandlerSetter) error {
	if _, exist := a.methodHandlers[method]; exist {
		return errors.New("handler already exist!")
	}
	h := &MethodHandler{
		callback: callback,
	}

	for _, setter := range setters {
		setter(h)
	}
	a.methodHandlers[method] = h
	return nil
}

// register a typed method handler
func (a *Actor) OnTyped(method string, typedHandler any, setters ...HandlerSetter) {
	handler, err := wrapTyped(typedHandler, nil)
	if err != nil {
		panic(err)
	}
	err = a.OnRequest(method, handler, setters...)
	if err != nil {
		panic(err)
	}
}

func (a *Actor) OnTypedRequest(method string, typedHandler any, setters ...HandlerSetter) error {
	//firstArg := reflect.TypeOf(&RPCRequest{})
	handler, err := wrapTyped(typedHandler, &ReqSpec{})
	if err != nil {
		return err
	}
	return a.OnRequest(method, handler, setters...)
}

func (a *Actor) OnTypedContext(method string, typedHandler any, setters ...HandlerSetter) error {
	//firstArgSpec := reflect.TypeOf((*context.Context)(nil)).Elem()
	handler, err := wrapTyped(typedHandler, &ContextSpec{})
	if err != nil {
		return err
	}
	return a.OnRequest(method, handler, setters...)
}

// Off unregister the method from handlers
func (a *Actor) Off(method string) {
	delete(a.methodHandlers, method)
}

// register a callback called when no hander to handle a request
// message or non-request message met
func (a *Actor) OnMissing(handler MissingCallback) error {
	if a.missingHandler != nil {
		return errors.New("missing handler already exist!")
	}
	a.missingHandler = handler
	return nil
}

// OnClose handler is called when the stream beneath the actor is closed
func (a *Actor) OnClose(handler CloseCallback) error {
	if a.closeHandler != nil {
		return errors.New("close handler already exist!")
	}
	a.closeHandler = handler
	return nil
}

// call the close handler if possible
func (a *Actor) HandleClose(session RPCSession) {
	// each child have to be called
	for _, child := range a.children {
		child.HandleClose(session)
	}

	if a.closeHandler != nil {
		a.closeHandler(session)
	}
}

// returns there is a handler for a method
func (a Actor) Has(method string) bool {
	if _, exist := a.methodHandlers[method]; exist {
		return true
	}

	for _, child := range a.children {
		if child.Has(method) {
			return true
		}
	}
	return false
}

func (a Actor) MethodList() []string {
	methods := []string{}
	for mname := range a.methodHandlers {
		methods = append(methods, mname)
	}
	for _, child := range a.children {
		childMethods := child.MethodList()
		methods = append(methods, childMethods...)
	}
	return methods
}

// get the schema of a method
func (a Actor) GetSchema(method string) (jsoffschema.Schema, bool) {
	if h, ok := a.getHandler(method); ok && h.schema != nil {
		return h.schema, true
	}
	for _, child := range a.children {
		if s, ok := child.GetSchema(method); ok {
			return s, ok
		}
	}
	return nil, false
}

// get a map of all supported schemas
func (a Actor) PublicSchemas() [](map[string]any) {
	methods := make([]map[string]any, 0)
	for _, mname := range a.MethodList() {
		if !jsoff.IsPublicMethod(mname) {
			continue
		}
		if s, ok := a.GetSchema(mname); ok {
			sMap := s.Map()
			sMap["name"] = mname
			methods = append(methods, sMap)
		} else {
			methods = append(methods, map[string]any{
				"name":        mname,
				"description": "",
				"params":      []any{},
				"returns": map[string]any{
					"type": "any",
				},
			})
		}
	}
	return methods
}

// get the handler of a method
func (a *Actor) getHandler(method string) (*MethodHandler, bool) {
	if h, ok := a.methodHandlers[method]; ok {
		return h, true
	} else {
		return nil, false
	}
}

// give the actor a request message
func (a *Actor) Feed(req *RPCRequest) (jsoff.Message, error) {
	msg := req.Msg()
	if !msg.IsResponse() {
		if a.missingHandler != nil {
			res, err := a.missingHandler(req)
			return a.wrapResult(res, err, msg)
		} else {
			req.Log().Info("no handler to handle this message")
			return nil, nil
		}
	}

	// TODO: recover from panic
	if handler, found := a.getHandler(msg.MustMethod()); found {
		params := msg.MustParams()
		if handler.schema != nil && a.ValidateSchema {
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
		return a.recoverCallHandler(handler, req, params)
	} else {
		for _, child := range a.children {
			if child.Has(msg.MustMethod()) {
				return child.Feed(req)
			}
		}
		if a.missingHandler != nil {
			return a.recoverCallMissingHandler(req)
		} else {
			if msg.IsRequest() {
				return jsoff.ErrMethodNotFound.ToMessageFromId(
					msg.MustId(), msg.TraceId()), nil
			}
		}
	}
	return nil, nil
}

func (a Actor) recoverCallHandler(handler *MethodHandler, req *RPCRequest, params []any) (resmsg0 jsoff.Message, err0 error) {
	if a.RecoverFromPanic {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					resmsg0, err0 = a.wrapResult(nil, err, req.Msg())
				} else {
					panic(r)
				}
			}
		}()
	}
	res, err := handler.callback(req, params)
	return a.wrapResult(res, err, req.Msg())
}

func (a Actor) recoverCallMissingHandler(req *RPCRequest) (resmsg0 jsoff.Message, err0 error) {
	if a.RecoverFromPanic {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					resmsg0, err0 = a.wrapResult(nil, err, req.Msg())
				} else {
					// rethrown the panic result
					panic(r)
				}
			}
		}()
	}
	res, err := a.missingHandler(req)
	return a.wrapResult(res, err, req.Msg())
}

func (a Actor) wrapResult(res any, err error, msg jsoff.Message) (jsoff.Message, error) {
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

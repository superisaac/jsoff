package jsonzhttp

import (
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"github.com/superisaac/jsonz/schema"
	"net/http"
)

const (
	TransportHTTP      = "http"
	TransportWebsocket = "websocket"
	TransportHTTP2     = "http2"
)

type RPCSession interface {
	Send(msg jsonz.Message)
	SessionID() string
}

// http rpc quest structure
type RPCRequest struct {
	context       context.Context
	msg           jsonz.Message
	transportType string
	r             *http.Request
	data          interface{} // arbitrary data
	session       RPCSession
}

func NewRPCRequest(ctx context.Context, msg jsonz.Message, transportType string, r *http.Request) *RPCRequest {
	return &RPCRequest{
		context:       ctx,
		msg:           msg,
		transportType: transportType,
		r:             r,
	}
}

func (self RPCRequest) Context() context.Context {
	return self.context
}

func (self RPCRequest) Msg() jsonz.Message {
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
type HandlerCallback func(req *RPCRequest, params []interface{}) (interface{}, error)
type MissingCallback func(req *RPCRequest) (interface{}, error)
type CloseCallback func(r *http.Request, session RPCSession)

// With method handler
type MethodHandler struct {
	callback HandlerCallback
	schema   jsonzschema.Schema
}

type HandlerSetter func(h *MethodHandler)

func WithSchema(s jsonzschema.Schema) HandlerSetter {
	return func(h *MethodHandler) {
		h.schema = s
	}
}

func WithSchemaYaml(yamlSchema string) HandlerSetter {
	builder := jsonzschema.NewSchemaBuilder()
	s, err := builder.BuildYamlBytes([]byte(yamlSchema))
	if err != nil {
		panic(err)
	}
	return WithSchema(s)
}

func WithSchemaJson(jsonSchema string) HandlerSetter {
	builder := jsonzschema.NewSchemaBuilder()
	s, err := builder.BuildBytes([]byte(jsonSchema))
	if err != nil {
		panic(err)
	}
	return WithSchema(s)
}

type Actor struct {
	ValidateSchema bool
	methodHandlers map[string]*MethodHandler
	missingHandler MissingCallback
	closeHandler   CloseCallback
}

func NewActor() *Actor {
	return &Actor{
		ValidateSchema: true,
		methodHandlers: make(map[string]*MethodHandler),
	}
}

// register a method handler
func (self *Actor) On(method string, callback HandlerCallback, setters ...HandlerSetter) error {
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
func (self *Actor) OnTyped(method string, typedHandler interface{}, setters ...HandlerSetter) error {
	firstArg := &RPCRequest{}
	handler, err := wrapTyped(typedHandler, firstArg)
	if err != nil {
		return err
	}
	return self.On(method, handler, setters...)
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
func (self *Actor) HandleClose(r *http.Request, session RPCSession) {
	if self.closeHandler != nil {
		self.closeHandler(r, session)
	}
}

// returns there is a handler for a method
func (self Actor) Has(method string) bool {
	_, exist := self.methodHandlers[method]
	return exist
}

func (self Actor) MethodList() []string {
	methods := []string{}
	for mname, _ := range self.methodHandlers {
		methods = append(methods, mname)
	}
	return methods
}

// get the schema of a method
func (self Actor) GetSchema(method string) (jsonzschema.Schema, bool) {
	if h, ok := self.getHandler(method); ok && h.schema != nil {
		return h.schema, true
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
func (self *Actor) Feed(req *RPCRequest) (jsonz.Message, error) {
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
			validator := jsonzschema.NewSchemaValidator()
			m, err := jsonz.MessageMap(msg)
			if err != nil {
				return nil, err
			}
			errPos := validator.Validate(handler.schema, m)
			if errPos != nil {
				if reqmsg, ok := msg.(*jsonz.RequestMessage); ok {
					return errPos.ToMessage(reqmsg), nil
				}
				return nil, errPos
			}
		}
		res, err := handler.callback(req, params)
		resmsg, err := self.wrapResult(res, err, msg)
		return resmsg, err
	} else if self.missingHandler != nil {
		res, err := self.missingHandler(req)
		resmsg, err := self.wrapResult(res, err, msg)
		return resmsg, err
	} else {
		if msg.IsRequest() {
			return jsonz.ErrMethodNotFound.ToMessageFromId(
				msg.MustId(), msg.TraceId()), nil
		}
	}
	return nil, nil
}

func (self Actor) wrapResult(res interface{}, err error, msg jsonz.Message) (jsonz.Message, error) {
	if !msg.IsRequest() {
		if err != nil {
			msg.Log().Warnf("error %s", err)
		}
		return nil, err
	}

	reqmsg, ok := msg.(*jsonz.RequestMessage)
	if !ok {
		msg.Log().Panicf("convert to request message failed")
		return nil, err
	}

	if err != nil {
		var rpcErr *jsonz.RPCError
		if errors.As(err, &rpcErr) {
			return rpcErr.ToMessage(reqmsg), nil
		} else {
			return jsonz.ErrInternalError.ToMessage(reqmsg), nil
		}
	} else if resmsg1, ok := res.(jsonz.Message); ok {
		// normal response
		return resmsg1, nil
	} else {
		return jsonz.NewResultMessage(reqmsg, res), nil
	}
}

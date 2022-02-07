package jsonzhttp

import (
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonz"
	"net/http"
)

// http rpc quest structure
type RPCRequest struct {
	context context.Context
	msg     jsonz.Message
	r       *http.Request
	data    interface{} // arbitrary data
}

func NewRPCRequest(ctx context.Context, msg jsonz.Message, r *http.Request, data interface{}) *RPCRequest {
	return &RPCRequest{context: ctx, msg: msg, r: r, data: data}
}

func (self RPCRequest) Context() context.Context {
	return self.context
}

func (self RPCRequest) Msg() jsonz.Message {
	return self.msg
}

func (self RPCRequest) HttpRequest() *http.Request {
	return self.r
}

func (self RPCRequest) Data() interface{} {
	return self.data
}

// handler func
type HandlerFunc func(req *RPCRequest, params []interface{}) (interface{}, error)
type MissingHandlerFunc func(req *RPCRequest) (interface{}, error)
type CloseHandlerFunc func(r *http.Request)

type Handler struct {
	methodHandlers map[string]HandlerFunc
	missingHandler MissingHandlerFunc
	closeHandler   CloseHandlerFunc
}

func NewHandler() *Handler {
	return &Handler{
		methodHandlers: make(map[string]HandlerFunc),
	}
}

func (self *Handler) On(method string, handler HandlerFunc) error {
	if _, exist := self.methodHandlers[method]; exist {
		return errors.New("handler already exist!")
	}
	self.methodHandlers[method] = handler
	return nil
}

func (self *Handler) OnTyped(method string, typedHandler interface{}) error {
	handler, err := wrapTyped(typedHandler)
	if err != nil {
		return err
	}
	return self.On(method, handler)
}

func (self *Handler) OnMissing(handler MissingHandlerFunc) error {
	if self.missingHandler != nil {
		return errors.New("missing handler already exist!")
	}
	self.missingHandler = handler
	return nil
}

func (self *Handler) OnClose(handler CloseHandlerFunc) error {
	if self.closeHandler != nil {
		return errors.New("close handler already exist!")
	}
	self.closeHandler = handler
	return nil
}

func (self *Handler) HandleClose(r *http.Request) {
	if self.closeHandler != nil {
		self.closeHandler(r)
	}
}

func (self Handler) HasHandler(method string) bool {
	_, exist := self.methodHandlers[method]
	return exist
}

func (self *Handler) getHandler(method string) (HandlerFunc, bool) {
	if h, ok := self.methodHandlers[method]; ok {
		return h, true
	} else {
		return nil, false
	}
}

func (self *Handler) HandleRequest(req *RPCRequest) (jsonz.Message, error) {
	msg := req.Msg()
	if !msg.IsRequestOrNotify() {
		if self.missingHandler != nil {
			res, err := self.missingHandler(req)
			return self.wrapResult(res, err, msg)
		} else {
			msg.Log().Info("no handler to handle this message")
			return nil, nil
		}
	}

	// TODO: recover from panic
	if handler, found := self.getHandler(msg.MustMethod()); found {
		params := msg.MustParams()
		res, err := handler(req, params)
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

func (self Handler) wrapResult(res interface{}, err error, msg jsonz.Message) (jsonz.Message, error) {
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
	return nil, nil
}

package jsozhttp

import (
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsoz"
	"net/http"
)

// rpc context
type RPCRequest struct {
	context context.Context
	msg     jsoz.Message
	r       *http.Request
	data    interface{} // arbitrary data
}

func (self RPCRequest) Context() context.Context {
	return self.context
}

func (self RPCRequest) Msg() jsoz.Message {
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

type Router struct {
	methodHandlers map[string]HandlerFunc
	missingHandler MissingHandlerFunc
	closeHandler   CloseHandlerFunc
}

func NewRouter() *Router {
	return &Router{
		methodHandlers: make(map[string]HandlerFunc),
	}
}

func (self *Router) On(method string, handler HandlerFunc) error {
	if _, exist := self.methodHandlers[method]; exist {
		return errors.New("handler already exist!")
	}
	self.methodHandlers[method] = handler
	return nil
}

func (self *Router) OnTyped(method string, typedHandler interface{}) error {
	handler, err := wrapTyped(typedHandler)
	if err != nil {
		return err
	}
	return self.On(method, handler)
}

func (self *Router) OnMissing(handler MissingHandlerFunc) error {
	if self.missingHandler != nil {
		return errors.New("missing handler already exist!")
	}
	self.missingHandler = handler
	return nil
}

func (self *Router) OnClose(handler CloseHandlerFunc) error {
	if self.closeHandler != nil {
		return errors.New("close handler already exist!")
	}
	self.closeHandler = handler
	return nil
}

func (self Router) HasHandler(method string) bool {
	_, exist := self.methodHandlers[method]
	return exist
}

func (self *Router) getHandler(method string) (HandlerFunc, bool) {
	if h, ok := self.methodHandlers[method]; ok {
		return h, true
	} else {
		return nil, false
	}
}

func (self *Router) handleRequest(req *RPCRequest) (jsoz.Message, error) {
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
			return jsoz.ErrMethodNotFound.ToMessageFromId(msg.MustId(), msg.TraceId()), nil
		}
	}
	return nil, nil
}

func (self Router) wrapResult(res interface{}, err error, msg jsoz.Message) (jsoz.Message, error) {
	if !msg.IsRequest() {
		if err != nil {
			msg.Log().Warnf("error %s", err)
		}
		return nil, err
	}

	reqmsg, ok := msg.(*jsoz.RequestMessage)
	if !ok {
		msg.Log().Panicf("convert to request message failed")
		return nil, err
	}

	if err != nil {
		var rpcErr *jsoz.RPCError
		if errors.As(err, &rpcErr) {
			return rpcErr.ToMessage(reqmsg), nil
		} else {
			return jsoz.ErrInternalError.ToMessage(reqmsg), nil
		}
	} else if resmsg1, ok := res.(jsoz.Message); ok {
		// normal response
		return resmsg1, nil
	} else {
		return jsoz.NewResultMessage(reqmsg, res), nil
	}
	return nil, nil
}

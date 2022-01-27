package jsonrpchttp

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonrpc"
	"net/http"
)

// rpc context
type RPCRequest struct {
	context context.Context
	msg     jsonrpc.IMessage
	r       *http.Request
}

func (self RPCRequest) Context() context.Context {
	return self.context
}

func (self RPCRequest) Msg() jsonrpc.IMessage {
	return self.msg
}

func (self RPCRequest) HttpRequest() *http.Request {
	return self.r
}

type BearHttpResponse struct {
	Code int
	Body []byte
}

func (self BearHttpResponse) Error() string {
	return fmt.Sprintf("%d/%s", self.Code, self.Body)
}

// handler func
type HandlerFunc func(req *RPCRequest, params []interface{}) (interface{}, error)
type MissingHandlerFunc func(req *RPCRequest) (interface{}, error)

type Router struct {
	methodHandlers map[string]HandlerFunc
	missingHandler MissingHandlerFunc
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

func (self *Router) handleMessage(rootCtx context.Context, msg jsonrpc.IMessage, r *http.Request) (jsonrpc.IMessage, error) {
	req := &RPCRequest{context: rootCtx, msg: msg, r: r}
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
			return jsonrpc.ErrMethodNotFound.ToMessageFromId(msg.MustId(), msg.TraceId()), nil
		}
	}
	return nil, nil
}

func (self Router) wrapResult(res interface{}, err error, msg jsonrpc.IMessage) (jsonrpc.IMessage, error) {
	if !msg.IsRequest() {
		if err != nil {
			msg.Log().Warnf("error %s", err)
		}
		return nil, err
	}

	reqmsg, ok := msg.(*jsonrpc.RequestMessage)
	if !ok {
		msg.Log().Panicf("convert to request message failed")
		return nil, err
	}

	if err != nil {
		var rpcErr *jsonrpc.RPCError
		if errors.As(err, &rpcErr) {
			return rpcErr.ToMessage(reqmsg), nil
		} else {
			return jsonrpc.ErrInternalError.ToMessage(reqmsg), nil
		}
	} else if resmsg1, ok := res.(jsonrpc.IMessage); ok {
		// normal response
		return resmsg1, nil
	} else {
		return jsonrpc.NewResultMessage(reqmsg, res), nil
	}
	return nil, nil
}

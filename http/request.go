package jsonrpchttp

import (
	//"fmt"
	"context"
	"github.com/pkg/errors"
	"github.com/superisaac/jsonrpc"
)

// rpc context
type RPCRequest struct {
	context context.Context
	msg     jsonrpc.IMessage
}

func (self RPCRequest) Context() context.Context {
	return self.context
}
func (self RPCRequest) Msg() jsonrpc.IMessage {
	return self.msg
}

// handler func
type HandlerFunc func(req *RPCRequest, params []interface{}) (interface{}, error)
type MissingHandlerFunc func(req *RPCRequest) (interface{}, error)

type Dispatcher struct {
	methodHandlers map[string]HandlerFunc
	missingHandler MissingHandlerFunc
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		methodHandlers: make(map[string]HandlerFunc),
	}
}

func (self *Dispatcher) On(method string, handler HandlerFunc) error {
	if _, exist := self.methodHandlers[method]; exist {
		return errors.New("handler already exist!")
	}
	self.methodHandlers[method] = handler
	return nil
}

func (self *Dispatcher) OnTyped(method string, typedHandler interface{}) error {
	handler, err := wrapTyped(typedHandler)
	if err != nil {
		return err
	}
	return self.On(method, handler)
}

func (self *Dispatcher) OnMissing(handler MissingHandlerFunc) error {
	if self.missingHandler != nil {
		return errors.New("missing handler already exist!")
	}
	self.missingHandler = handler
	return nil
}

func (self Dispatcher) HasHandler(method string) bool {
	_, exist := self.methodHandlers[method]
	return exist
}

func (self *Dispatcher) getHandler(method string) (HandlerFunc, bool) {
	if h, ok := self.methodHandlers[method]; ok {
		return h, true
	} else {
		return nil, false
	}
}

func (self *Dispatcher) handleMessage(rootCtx context.Context, msg jsonrpc.IMessage) (jsonrpc.IMessage, error) {
	if !msg.IsRequestOrNotify() {
		msg.Log().Warnf("handler only accept request and notify")
		return nil, errors.New("bad msg type")
	}

	// TODO: recover from panic

	if handler, found := self.getHandler(msg.MustMethod()); found {
		//ctx, cancel := context.WithCancel(rootCtx)
		//defer cancel()
		req := &RPCRequest{context: rootCtx, msg: msg}
		params := msg.MustParams()
		res, err := handler(req, params)
		resmsg, err := self.wrapResult(res, err, msg)
		return resmsg, err
	} else if self.missingHandler != nil {
		req := &RPCRequest{context: rootCtx, msg: msg}
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

func (self Dispatcher) wrapResult(res interface{}, err error, msg jsonrpc.IMessage) (jsonrpc.IMessage, error) {
	var reqmsg *jsonrpc.RequestMessage
	if msg.IsRequest() {
		reqmsg, _ = msg.(*jsonrpc.RequestMessage)
	}
	if err != nil {

		if msg.IsRequest() {
			var rpcErr *jsonrpc.RPCError
			if errors.As(err, &rpcErr) {
				return rpcErr.ToMessage(reqmsg), nil
			} else {
				return jsonrpc.ErrInternalError.ToMessage(reqmsg), nil
			}
		} else {
			msg.Log().Warnf("error %s", err)
		}
	} else if resmsg1, ok := res.(jsonrpc.IMessage); ok {
		// normal response
		return resmsg1, nil
	} else if reqmsg != nil {
		return jsonrpc.NewResultMessage(reqmsg, res), nil
	}
	return nil, nil
}

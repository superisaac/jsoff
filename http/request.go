package jsonrpchttp

import (
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

type Dispatcher struct {
	methodHandlers map[string]HandlerFunc
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		methodHandlers: make(map[string]HandlerFunc),
	}
}

func (self *Dispatcher) On(method string, handler HandlerFunc) error {
	if _, exist := self.methodHandlers[method]; exist {
		return errors.New("handler already exist")
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

func (self Dispatcher) HasHandler(method string) bool {
	_, exist := self.methodHandlers[method]
	return exist
}

func (self *Dispatcher) getHandler(method string) (HandlerFunc, bool) {
	if h, ok := self.methodHandlers[method]; ok {
		return h, true
	} else if h, ok := self.methodHandlers["*"]; ok {
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

	var reqmsg *jsonrpc.RequestMessage

	if msg.IsRequest() {
		reqmsg, _ = msg.(*jsonrpc.RequestMessage)
	}

	if handler, found := self.getHandler(msg.MustMethod()); found {
		//ctx, cancel := context.WithCancel(rootCtx)
		//defer cancel()
		req := &RPCRequest{context: rootCtx, msg: msg}
		params := msg.MustParams()
		res, err := handler(req, params)
		if err != nil {
			if msg.IsRequest() {
				if rpcErr, ok := err.(*jsonrpc.RPCError); ok {
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
	} else {
		if msg.IsRequest() {
			return jsonrpc.ErrMethodNotFound.ToMessage(reqmsg), nil
		}
	}
	return nil, nil
}

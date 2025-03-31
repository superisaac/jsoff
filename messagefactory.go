package jsoff

import (
	"encoding/json"
)

type MessageFactory struct {
	opts MessageOptions
}

func NewMessageFactory(options ...MessageOptions) *MessageFactory {
	opts := MessageOptions{}

	if len(options) > 0 {
		opts = options[0]
	}
	return &MessageFactory{opts: opts}
}

func (f MessageFactory) Parse(data []byte) (Message, error) {
	return ParseBytes(data, f.opts)
}

func (f MessageFactory) Decode(decoder *json.Decoder) (Message, error) {
	return DecodeMessage(decoder, f.opts)
}

func (f MessageFactory) NewRequest(id any, method string, params any) *RequestMessage {
	if f.opts.IdNotNull && id == nil {
		panic("Request id cannot be null")
	}
	return NewRequestMessage(id, method, params)
}

func (f MessageFactory) NewNotify(method string, params any) *NotifyMessage {
	return NewNotifyMessage(method, params)
}

func (f MessageFactory) NewResult(reqmsg Message, result any) *ResultMessage {
	if f.opts.IdNotNull && (reqmsg == nil || reqmsg.MustId() == nil) {
		panic("Result id cannot be null")
	}
	return NewResultMessage(reqmsg, result)
}

func (f MessageFactory) NewError(reqmsg Message, errbody *RPCError) *ErrorMessage {
	if f.opts.IdNotNull && (reqmsg == nil || reqmsg.MustId() == nil) {
		panic("Error id cannot be null")
	}
	return NewErrorMessage(reqmsg, errbody)
}

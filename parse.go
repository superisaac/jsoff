package jsonrpc

import (
	"github.com/bitly/go-simplejson"
	"github.com/pkg/errors"
)

func parseRPCError(errIntf *simplejson.Json) (*RPCError, error) {
	code, err := errIntf.Get("code").Int()
	if err != nil {
		return nil, errors.Wrap(err, ".Get(code).Int")
	}

	message, err := errIntf.Get("message").String()
	if err != nil {
		return nil, errors.Wrap(err, ".Get(code).String")
	}

	data := errIntf.Get("data").Interface()
	return &RPCError{code, message, data}, nil
}

func ParseBytes(data []byte) (IMessage, error) {
	parsed, err := simplejson.NewJson(data)
	if err != nil {
		return nil, errors.Wrap(err, "simplejson.NewJson")
	}
	return Parse(parsed)
}

func parseParams(parsed *simplejson.Json) ([]interface{}, bool, error) {
	if arr, err := parsed.Array(); err == nil {
		return arr, true, nil
	} else if obj, err := parsed.Map(); err == nil {
		return [](interface{}){obj}, false, nil
	} else {
		return nil, false, errors.New("params is neither array nor map")
	}
}

func Parse(parsed *simplejson.Json) (IMessage, error) {
	id := parsed.Get("id").Interface()
	method, err := parsed.Get("method").String()
	if err != nil {
		method = ""
	}

	traceId, err := parsed.Get("traceid").String()
	if err != nil {
		traceId = ""
	}

	if id != nil {
		if method != "" {
			// request
			params, paramsAreList, err := parseParams(parsed.Get("params"))
			if err != nil {
				return nil, err
			}
			reqmsg := NewRequestMessage(id, method, params)
			reqmsg.paramsAreList = paramsAreList
			reqmsg.SetRaw(parsed)
			reqmsg.SetTraceId(traceId)
			return reqmsg, nil
		}
		if errIntf := parsed.Get("error"); errIntf != nil && errIntf.Interface() != nil {
			errbody, err := parseRPCError(errIntf)
			if err != nil {
				return nil, err
			}
			errmsg := rawErrorMessage(id, errbody)
			errmsg.SetRaw(parsed)
			errmsg.SetTraceId(traceId)
			return errmsg, nil
		}
		res := parsed.Get("result").Interface()
		rmsg := rawResultMessage(id, res)
		rmsg.SetRaw(parsed)
		rmsg.SetTraceId(traceId)
		return rmsg, nil
	} else if method != "" {
		params, paramsAreList, err := parseParams(parsed.Get("params"))
		if err != nil {
			return nil, err
		}
		ntfmsg := NewNotifyMessage(method, params)
		ntfmsg.paramsAreList = paramsAreList
		ntfmsg.SetRaw(parsed)
		ntfmsg.SetTraceId(traceId)
		return ntfmsg, nil
	} else {
		return nil, ErrParseMessage
	}
}

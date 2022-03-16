package jsonz

import (
	"bytes"
	"encoding/json"
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

func ParseBytes(data []byte) (Message, error) {
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

func Parse(parsed *simplejson.Json) (Message, error) {
	id := parsed.Get("id").Interface()
	if numId, ok := id.(json.Number); ok {
		intId, err := numId.Int64()
		if err != nil {
			return nil, err
		}
		id = int(intId)
	}
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

type msgUnion struct {
	Id      *json.RawMessage `json:"id,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *json.RawMessage `json:"error,omitempty"`
	Params  *json.RawMessage `json:"params,omitempty"`
	Method  string           `json:"method,omitempty"`
	TraceId string           `json:"traceid,omitempty"`
}

type decodeErrorT struct {
	errmsg string
}

func (self decodeErrorT) Error() string {
	return "error decode: " + self.errmsg
}

func errdecode(errmsg string) *decodeErrorT {
	return &decodeErrorT{errmsg: errmsg}
}

func decodeId(un *msgUnion) (interface{}, error) {
	if un.Id == nil {
		// no id
		return nil, nil
	}

	// decode id
	var intId int
	if err := json.Unmarshal(*un.Id, &intId); err == nil {
		return intId, nil
	}

	var sid string
	if err := json.Unmarshal(*un.Id, &sid); err != nil {
		return nil, err
	}

	return sid, nil
}

func decodeParams(un *msgUnion) (p []interface{}, islist bool, e error) {
	if un.Params == nil {
		return []interface{}{}, true, nil
	}
	arr := []interface{}{}
	dec := json.NewDecoder(bytes.NewReader(*un.Params))
	dec.UseNumber()
	if err := dec.Decode(&arr); err != nil {
		var intfparams interface{}
		idec := json.NewDecoder(bytes.NewReader(*un.Params))
		idec.UseNumber()
		if err := idec.Decode(&intfparams); err != nil {
			return nil, false, err

		}
		return []interface{}{intfparams}, false, nil
	}
	return arr, true, nil
}

func DecodeMessage(decoder *json.Decoder) (Message, error) {
	var un msgUnion
	if err := decoder.Decode(&un); err != nil {
		return nil, err
	}

	if un.Error != nil {
		// senity check
		if un.Result != nil {
			return nil, errdecode("result and error cannot co exist")
		}
		// parse error body
		var errbody RPCError
		errdec := json.NewDecoder(bytes.NewReader(*un.Error))
		errdec.UseNumber()
		if err := errdec.Decode(&errbody); err != nil {
			return nil, err
		}

		id, err := decodeId(&un)
		if err != nil {
			return nil, err
		}

		errmsg := rawErrorMessage(id, &errbody)
		errmsg.SetTraceId(un.TraceId)
		return errmsg, nil
	} else if un.Result != nil {
		if un.Error != nil {
			return nil, errdecode("result and error cannot co exist")
		}

		// parse result
		var res interface{}
		resdec := json.NewDecoder(bytes.NewReader(*un.Result))
		resdec.UseNumber()
		if err := resdec.Decode(&res); err != nil {
			return nil, err
		}

		id, err := decodeId(&un)
		if err != nil {
			return nil, err
		}

		resmsg := rawResultMessage(id, res)
		resmsg.SetTraceId(un.TraceId)
		return resmsg, nil
	} else if un.Method != "" {
		params, islist, err := decodeParams(&un)
		if err != nil {
			return nil, err
		}

		id, err := decodeId(&un)
		if err != nil {
			return nil, err
		}

		if id != nil {
			reqmsg := NewRequestMessage(id, un.Method, params)
			reqmsg.paramsAreList = islist
			reqmsg.SetTraceId(un.TraceId)
			return reqmsg, nil
		} else {
			ntfmsg := NewNotifyMessage(un.Method, params)
			ntfmsg.paramsAreList = islist
			ntfmsg.SetTraceId(un.TraceId)
			return ntfmsg, nil
		}
	}
	return nil, errdecode("not a jsonrpc message")
}

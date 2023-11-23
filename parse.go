package jsoff

import (
	"bytes"
	"encoding/json"
)

func ParseBytes(data []byte) (Message, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	return DecodeMessage(decoder)
}

type msgUnion struct {
	IdSt    msgIdT           `json:"id,omitempty"`
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

func decodeParams(un *msgUnion) (p []interface{}, islist bool, e error) {
	if un.Params == nil {
		return nil, false, errdecode("no params field")
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
	decoder.UseNumber()
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

		if !un.IdSt.isSet {
			return nil, errdecode("no message id")
		}
		// id, err := decodeId(&un)
		// if err != nil {
		// 	return nil, err
		// }

		errmsg := rawErrorMessage(un.IdSt.Value, &errbody)
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

		var msgId interface{} = nil
		if un.IdSt.isSet {
			msgId = un.IdSt.Value
		}

		resmsg := rawResultMessage(msgId, res)
		resmsg.SetTraceId(un.TraceId)
		return resmsg, nil
	} else if un.Method != "" {
		params, islist, err := decodeParams(&un)
		if err != nil {
			return nil, err
		}

		if un.IdSt.isSet {
			reqmsg := NewRequestMessage(un.IdSt.Value, un.Method, params)
			reqmsg.paramsAreList = islist
			reqmsg.SetTraceId(un.TraceId)
			return reqmsg, nil
		} else {
			ntfmsg := NewNotifyMessage(un.Method, params)
			ntfmsg.paramsAreList = islist
			ntfmsg.SetTraceId(un.TraceId)
			return ntfmsg, nil
		}
	} else if un.IdSt.isSet {
		// parse id
		// id, err := decodeId(&un)
		// if err != nil {
		// 	return nil, err
		// }

		// result is null
		resmsg := rawResultMessage(un.IdSt.Value, nil)
		resmsg.SetTraceId(un.TraceId)
		return resmsg, nil
	}
	return nil, errdecode("not a jsonrpc message")
}

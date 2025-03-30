package jsoff

import (
	"encoding/json"
)

// hold a string|integer|null id
type msgIdT struct {
	Value any // value can be string, integer and null
	isSet bool
}

func (mt msgIdT) MarshalJSON() ([]byte, error) {
	if mt.Value == nil && mt.isSet {
		return []byte("null"), nil
	}
	return json.Marshal(mt.Value)
}

func (mt *msgIdT) UnmarshalJSON(data []byte) error {
	// try null value
	if string(data) == "null" {
		mt.Value = nil
		mt.isSet = true
		return nil
	}
	// try int value
	var intv int
	if err := json.Unmarshal(data, &intv); err == nil {
		mt.Value = intv
		mt.isSet = true
		return nil
	}

	// try string value
	var strv string
	if err := json.Unmarshal(data, &strv); err != nil {
		return err
	}

	mt.Value = strv
	mt.isSet = true
	return nil
}

// marshaling templates
type templateRequest struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Id      msgIdT `json:"id"`
	Params  any    `json:"params"`
	TraceId string `json:"traceid,omitempty"`
}

type templateNotify struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	TraceId string `json:"traceid,omitempty"`
}

type templateResult struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      msgIdT `json:"id"`
	Result  any    `json:"result"`
	TraceId string `json:"traceid,omitempty"`
}

type templateError struct {
	Jsonrpc string    `json:"jsonrpc"`
	Id      msgIdT    `json:"id"`
	Error   *RPCError `json:"error"`
	TraceId string    `json:"traceid,omitempty"`
}

package jsoff

import (
	"encoding/json"
)

// hold a string|integer|null id
type msgIdT struct {
	Value interface{} // value can be string, integer and null
	isSet bool
}

func (self msgIdT) MarshalJSON() ([]byte, error) {
	if self.Value == nil && self.isSet {
		return []byte("null"), nil
	}
	return json.Marshal(self.Value)
}

func (self *msgIdT) UnmarshalJSON(data []byte) error {
	// try null value
	if string(data) == "null" {
		self.Value = nil
		self.isSet = true
		return nil
	}
	// try int value
	var intv int
	if err := json.Unmarshal(data, &intv); err == nil {
		self.Value = intv
		self.isSet = true
		return nil
	}

	// try string value
	var strv string
	if err := json.Unmarshal(data, &strv); err != nil {
		return err
	}

	self.Value = strv
	self.isSet = true
	return nil
}

// marshaling templates
type templateRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Id      msgIdT      `json:"id"`
	Params  interface{} `json:"params"`
	TraceId string      `json:"traceid,omitempty"`
}

type templateNotify struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	TraceId string      `json:"traceid,omitempty"`
}

type templateResult struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      msgIdT      `json:"id"`
	Result  interface{} `json:"result"`
	TraceId string      `json:"traceid,omitempty"`
}

type templateError struct {
	Jsonrpc string    `json:"jsonrpc"`
	Id      msgIdT    `json:"id"`
	Error   *RPCError `json:"error"`
	TraceId string    `json:"traceid,omitempty"`
}

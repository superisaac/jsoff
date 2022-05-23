package jlib

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"math/big"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

func MarshalJson(data interface{}) (string, error) {
	marshaled, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(marshaled), nil
}

func GuessJson(input string) (interface{}, error) {
	if len(input) == 0 {
		return "", nil
	}
	if input == "true" || input == "false" {
		bv, _ := strconv.ParseBool(input)
		return bv, nil
	}

	iv, err := strconv.ParseInt(input, 10, 64)
	if err == nil {
		return iv, nil
	}
	fv, err := strconv.ParseFloat(input, 64)
	if err == nil {
		return fv, nil
	}

	fc := input[0]
	if fc == '[' {
		var arr []interface{}
		dec := json.NewDecoder(strings.NewReader(input))
		dec.UseNumber()
		err := dec.Decode(&arr)
		if err != nil {
			return nil, err
		}
		return arr, nil
	} else if fc == '{' {
		var m map[string]interface{}
		dec := json.NewDecoder(strings.NewReader(input))
		dec.UseNumber()
		err := dec.Decode(&m)
		if err != nil {
			return nil, err
		}
		return m, nil
	} else if fc == '"' {
		var s string
		dec := json.NewDecoder(strings.NewReader(input))
		dec.UseNumber()
		err := dec.Decode(&s)
		if err != nil {
			return nil, err
		}
		return s, nil
	} else {
		return input, nil
	}
}

func GuessJsonArray(inputArr []string) ([]interface{}, error) {
	var arr []interface{}
	for _, input := range inputArr {
		v, err := GuessJson(input)
		if err != nil {
			return arr, err
		}
		arr = append(arr, v)
	}
	return arr, nil
}

func ErrorResponse(w http.ResponseWriter, r *http.Request, err error, status int, message string) {
	log.Warnf("HTTP error: %s %d", err.Error(), status)
	w.WriteHeader(status)
	w.Write([]byte(message))
}

func NewUuid() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func DecodeInterface(input interface{}, output interface{}) error {
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		TagName:  "json",
		Result:   output,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return errors.Wrap(err, "decode interface")
	}
	return decoder.Decode(input)
}

/// Convert params to a struct field by field
func DecodeParams(params []interface{}, outputPtr interface{}) error {
	ptrType := reflect.TypeOf(outputPtr)
	if ptrType.Kind() != reflect.Ptr {
		return errors.New("output is not a pointer")
	}
	outputType := ptrType.Elem()
	if outputType.Kind() != reflect.Struct {
		return errors.New("output is not pointer of struct")
	}

	fields := reflect.VisibleFields(outputType)
	ptrValue := reflect.ValueOf(outputPtr)
	idx := 0
	for _, field := range fields {
		if !field.IsExported() {
			continue
		}
		if len(params) <= idx {
			continue
		}
		param := params[idx]
		idx++
		ov := reflect.Zero(field.Type).Interface()
		config := &mapstructure.DecoderConfig{
			Metadata: nil,
			TagName:  "json",
			Result:   &ov,
		}
		decoder, err := mapstructure.NewDecoder(config)
		if err != nil {
			return errors.Wrap(err, "NewDecoder")
		}
		err = decoder.Decode(param)
		if err != nil {
			return errors.Wrap(err, "mapstruct.Decode")
		}

		ptrValue.Elem().FieldByIndex(field.Index).Set(
			reflect.ValueOf(ov))
	}
	return nil
}

type Bigint big.Int

func (self Bigint) MarshalJSON() ([]byte, error) {
	bi := big.Int(self)
	return []byte(fmt.Sprintf(`"%s"`, bi.String())), nil
}

func (self *Bigint) UnmarshalJSON(data []byte) error {
	sd := string(data)
	bi := (*big.Int)(self)

	if strings.HasPrefix(sd, `"`) {
		// string repr
		bi.SetString(sd[1:len(sd)-1], 10)
	} else {
		bi.SetString(sd, 10)
	}
	return nil
}

func (self *Bigint) Value() *big.Int {
	return (*big.Int)(self)
}

func (self *Bigint) String() string {
	return self.Value().String()
}

func (self *Bigint) Load(repr string) {
	self.Value().SetString(repr, 10)
}

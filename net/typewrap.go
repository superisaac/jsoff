package jsoffnet

import (
	"context"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/superisaac/jsoff"
	"reflect"
)

func typeIsStruct(tp reflect.Type) bool {
	return (tp.Kind() == reflect.Struct ||
		(tp.Kind() == reflect.Ptr && typeIsStruct(tp.Elem())))
}

func interfaceToValue(a interface{}, outputType reflect.Type) (reflect.Value, error) {
	output := reflect.Zero(outputType).Interface()
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		TagName:  "json",
		Result:   &output,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return reflect.Value{}, err
	}
	err = decoder.Decode(a)
	if err != nil {
		return reflect.Value{}, err
	}
	return reflect.ValueOf(output), nil
}

func valueToInterface(tp reflect.Type, val reflect.Value) (interface{}, error) {
	var output interface{}
	if typeIsStruct(tp) {
		output = make(map[string]interface{})
	} else {
		output = reflect.Zero(tp).Interface()
	}
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		TagName:  "json",
		Result:   &output,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(val.Interface())
	if err != nil {
		return nil, err
	}
	return output, nil
}

type FirstArgSpec interface {
	Check(firstArgType reflect.Type) bool
	Value(req *RPCRequest) interface{}
	String() string
}

type ReqSpec struct{}

func (self ReqSpec) Check(firstArgType reflect.Type) bool {
	return firstArgType.Kind() == reflect.Ptr && firstArgType.String() == self.String()
}
func (self ReqSpec) Value(req *RPCRequest) interface{} {
	return req
}
func (self ReqSpec) String() string {
	return "*jsoffnet.RPCRequest"
}

type ContextSpec struct{}

func (self ContextSpec) Check(firstArgType reflect.Type) bool {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	return firstArgType.Kind() == reflect.Interface && firstArgType.Implements(ctxType)
}
func (self ContextSpec) Value(req *RPCRequest) interface{} {
	return req.Context()
}
func (self ContextSpec) String() string {
	return "context.Context"
}

func wrapTyped(tfunc interface{}, firstArgSpec FirstArgSpec) (RequestCallback, error) {

	funcType := reflect.TypeOf(tfunc)
	if funcType.Kind() != reflect.Func {
		return nil, errors.New("tfunc is not func type")
	}

	numIn := funcType.NumIn()

	requireFirstArg := firstArgSpec != (FirstArgSpec)(nil)
	firstArgNum := 0
	if requireFirstArg {
		firstArgNum = 1
		// check inputs and 1st argument
		if numIn < firstArgNum {
			return nil, errors.New("func must have 1 more arguments")
		}
		firstArgType := funcType.In(0)

		if !firstArgSpec.Check(firstArgType) {
			return nil, errors.New(fmt.Sprintf("the first arg must be %s", firstArgSpec.String()))
		}
	}

	// check outputs
	if funcType.NumOut() != 2 {
		return nil, errors.New("func return number must be 2")
	}

	errType := funcType.Out(1)
	errInterface := reflect.TypeOf((*error)(nil)).Elem()

	if !errType.Implements(errInterface) {
		return nil, errors.New("second output does not implement error")
	}

	handler := func(req *RPCRequest, params []interface{}) (interface{}, error) {
		// check inputs
		if numIn > len(params)+firstArgNum {
			return nil, jsoff.ParamsError("no enough params size")
		}

		// params -> []reflect.Value
		fnArgs := []reflect.Value{}
		if requireFirstArg {
			v := firstArgSpec.Value(req)
			fnArgs = append(fnArgs, reflect.ValueOf(v))
		}
		j := 0
		for i := firstArgNum; i < numIn; i++ {
			argType := funcType.In(i)
			param := params[j]
			j++

			argValue, err := interfaceToValue(param, argType)
			if err != nil {
				return nil, jsoff.ParamsError(
					fmt.Sprintf("params %d %s", i+1, err))
			}
			fnArgs = append(fnArgs, argValue)

		}

		// wrap result
		resValues := reflect.ValueOf(tfunc).Call(fnArgs)
		resType := funcType.Out(0)
		errRes := resValues[1].Interface()
		if errRes != nil {
			if err, ok := errRes.(error); ok {
				return nil, err
			} else {
				return nil, errors.New(fmt.Sprintf("error return is not error %+v", errRes))
			}
		}

		res, err := valueToInterface(
			resType, resValues[0])
		return res, err
	}

	return handler, nil
}

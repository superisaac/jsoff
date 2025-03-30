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

func interfaceToValue(a any, outputType reflect.Type) (reflect.Value, error) {
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

func valueToInterface(tp reflect.Type, val reflect.Value) (any, error) {
	var output any
	if typeIsStruct(tp) {
		output = make(map[string]any)
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
	Value(req *RPCRequest) any
	String() string
}

type ReqSpec struct{}

func (spec ReqSpec) Check(firstArgType reflect.Type) bool {
	return firstArgType.Kind() == reflect.Ptr && firstArgType.String() == spec.String()
}
func (spec ReqSpec) Value(req *RPCRequest) any {
	return req
}
func (spec ReqSpec) String() string {
	return "*jsoffnet.RPCRequest"
}

type ContextSpec struct{}

func (spec ContextSpec) Check(firstArgType reflect.Type) bool {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	return firstArgType.Kind() == reflect.Interface && firstArgType.Implements(ctxType)
}
func (spec ContextSpec) Value(req *RPCRequest) any {
	return req.Context()
}
func (spec ContextSpec) String() string {
	return "context.Context"
}

func wrapTyped(tfunc any, firstArgSpec FirstArgSpec) (RequestCallback, error) {

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

	handler := func(req *RPCRequest, params []any) (any, error) {
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

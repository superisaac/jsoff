package jsonrpc

import (
	json "encoding/json"
	"fmt"
	"regexp"
)

func ValidateNumber(v interface{}, prefix string) (json.Number, error) {
	if n, ok := v.(json.Number); ok {
		return n, nil
	} else {
		reason := fmt.Sprintf("%s requires a number", prefix)
		return json.Number("0"), &RPCError{10400, reason, false}
	}
}

func ValidateFloat(v interface{}, prefix string) (float64, error) {
	if f, ok := v.(float64); ok {
		return f, nil
	}
	n, err := ValidateNumber(v, prefix)
	if err != nil {
		return 0, err
	}
	f, err := n.Float64()
	if err != nil {
		reason := fmt.Sprintf("%s requires a float number", prefix)
		return 0, &RPCError{10900, reason, false}
	}
	return f, nil
}

func ValidateInt(v interface{}, prefix string) (int64, error) {
	if n, ok := v.(int64); ok {
		return n, nil
	}
	n, err := ValidateNumber(v, prefix)
	if err != nil {
		return 0, err
	}
	i, err := n.Int64()
	if err != nil {
		reason := fmt.Sprintf("%s requires an int number", prefix)
		return 0, &RPCError{11402, reason, false}
	}
	return i, nil
}

func ValidateString(v interface{}, prefix string) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	reason := fmt.Sprintf("%s require string", prefix)
	return "", &RPCError{11403, reason, false}
}

func MustNumber(input interface{}, prefix string) json.Number {
	v, err := ValidateNumber(input, prefix)
	if err != nil {
		panic(err)
	}
	return v
}

func MustFloat(input interface{}, prefix string) float64 {
	v, err := ValidateFloat(input, prefix)
	if err != nil {
		panic(err)
	}
	return v
}

func MustInt(input interface{}, prefix string) int64 {
	v, err := ValidateInt(input, prefix)
	if err != nil {
		panic(err)
	}
	return v
}

// method names
func IsMethod(name string) bool {
	matched, _ := regexp.MatchString(`^[0-9a-zA-Z\-\_\:\+\.]+$`, name)
	return matched
}

func IsPublicMethod(name string) bool {
	matched, _ := regexp.MatchString(`^[0-9a-zA-Z][0-9a-zA-Z\-\_\:\+\.]*$`, name)
	return matched
}

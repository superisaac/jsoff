package jsonz

import (
	simplejson "github.com/bitly/go-simplejson"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func MarshalJson(data interface{}) (string, error) {
	jsondata := simplejson.New()
	jsondata.SetPath(nil, data)
	bytes, err := jsondata.MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func GuessJson(input string) (interface{}, error) {
	if len(input) == 0 {
		return "", nil
	}
	if input == "true" || input == "false" {
		bv, err := strconv.ParseBool(input)
		if err != nil {
			return nil, err
		}
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
		parsed, err := simplejson.NewJson([]byte(input))
		if err != nil {
			return nil, err
		}
		return parsed.MustArray(), nil
	} else if fc == '{' {
		parsed, err := simplejson.NewJson([]byte(input))
		if err != nil {
			return nil, err
		}
		return parsed.MustMap(), nil
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

func ConvertString(v interface{}) string {
	if strv, ok := v.(string); ok {
		return strv
	}
	panic("cannot convert to string")
	//log.Fatalf("cannot convert %s to string", v)
	//return ""
}

func ConvertStringList(v interface{}) []string {
	if arr, ok := v.([]interface{}); ok {
		strarr := make([]string, 0)
		for _, a := range arr {
			s := ConvertString(a)
			strarr = append(strarr, s)
		}
		return strarr
	}
	panic("cannot convert to string array")
}

func ConvertInt(v interface{}) int {
	if intv, ok := v.(int); ok {
		return intv
	}
	panic("cannot convert to int")
}

func ErrorResponse(w http.ResponseWriter, r *http.Request, err error, status int, message string) {
	log.Warnf("HTTP error: %s %d", err.Error(), status)
	w.WriteHeader(status)
	w.Write([]byte(message))
}

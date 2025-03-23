package jsoffnet

import (
	"github.com/pkg/errors"
	"net/http"
	"strings"
)

type HeaderFlags []string

func (flag *HeaderFlags) String() string {
	return "header flags"
}

func (flag *HeaderFlags) Set(value string) error {
	*flag = append(*flag, value)
	return nil
}

func (flag *HeaderFlags) Parse() (http.Header, error) {
	header := make(http.Header)
	for _, hr := range *flag {
		arr := strings.SplitN(hr, ":", 2)
		if len(arr) != 2 {
			return nil, errors.New("invalid http header")
		}
		header.Add(strings.Trim(arr[0], " "), strings.Trim(arr[1], " "))
	}
	return header, nil
}

// // merge multiple http headers into one, may return nil
// func MergeHeaders(headers []http.Header) http.Header {
// 	var merged http.Header = nil
// 	for _, h := range headers {
// 		for k, vs := range h {
// 			for _, v := range vs {
// 				if merged == nil {
// 					merged = make(http.Header)
// 				}
// 				merged.Add(k, v)
// 			}
// 		}
// 	}
// 	return merged
// }

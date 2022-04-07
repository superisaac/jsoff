package jlibhttp

import (
	"fmt"
	"net/http"
)

// errors
// non standard Response returned by endpoints
type WrappedResponse struct {
	Response *http.Response
}

func (self WrappedResponse) Error() string {
	return fmt.Sprintf("wrapped response %d", self.Response.StatusCode)
}

// Simple HTTP response to instant return
type SimpleResponse struct {
	Code int
	Body []byte
}

func (self SimpleResponse) Error() string {
	return fmt.Sprintf("%d/%s", self.Code, self.Body)
}

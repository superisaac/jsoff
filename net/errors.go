package jsoffnet

import (
	"fmt"
	"net/http"
)

// errors
// non standard Response returned by endpoints
type WrappedResponse struct {
	Response *http.Response
	Body     []byte
}

func (resp WrappedResponse) Error() string {
	return fmt.Sprintf("wrapped response %d", resp.Response.StatusCode)
}

// Simple HTTP response to instant return
type SimpleResponse struct {
	Code int
	Body []byte
}

func (resp SimpleResponse) Error() string {
	return fmt.Sprintf("%d/%s", resp.Code, resp.Body)
}

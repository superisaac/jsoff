package jsozhttp

import (
	"fmt"
	"net/http"
)

// errors

// non standard Response returned by endpoints
type UpstreamResponse struct {
	Response *http.Response
}

func (self UpstreamResponse) Error() string {
	return fmt.Sprintf("upstream response %d", self.Response.StatusCode)
}

// Simple Http response to instant return
type SimpleHttpResponse struct {
	Code int
	Body []byte
}

func (self SimpleHttpResponse) Error() string {
	return fmt.Sprintf("%d/%s", self.Code, self.Body)
}

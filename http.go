package jsonrpc

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

func ErrorResponse(w http.ResponseWriter, r *http.Request, err error, status int, message string) {
	log.Warningf("HTTP error: %s %d", err.Error(), status)
	w.WriteHeader(status)
	w.Write([]byte(message))
}

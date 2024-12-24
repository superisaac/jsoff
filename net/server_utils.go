package jsoffnet

import (
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
)

type TLSConfig struct {
	Certfile string
	Keyfile  string
}

func (cfg *TLSConfig) ValidateValues() error {
	if cfg.Certfile == "" {
		return errors.New("certfile is empty")
	}
	if cfg.Keyfile == "" {
		return errors.New("keyfile is empty")
	}
	return nil
}

func ListenAndServe(rootCtx context.Context, bind string, handler http.Handler, tlsConfigs ...*TLSConfig) error {
	var tlsConfig *TLSConfig
	for _, cfg := range tlsConfigs {
		if cfg != nil {
			tlsConfig = cfg
			break
		}
	}
	server := &http.Server{Addr: bind, Handler: handler}
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}

	serverCtx, cancelServer := context.WithCancel(rootCtx)
	defer cancelServer()

	go func() {
		for {
			<-serverCtx.Done()
			listener.Close()
			return
		}
	}()

	if tlsConfig != nil {
		return server.ServeTLS(
			listener,
			tlsConfig.Certfile,
			tlsConfig.Keyfile)
	} else {
		err := server.Serve(listener)
		return errors.Wrap(err, "server.Serve")
	}
}

// log attaching remoteAddr
func Logger(r *http.Request) *log.Entry {
	return log.WithFields(log.Fields{
		"remoteAddr": r.RemoteAddr,
	})
}

package jsonzhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
)

type TLSConfig struct {
	Certfile string
	Keyfile  string
}

func (self *TLSConfig) ValidateValues() error {
	if self.Certfile == "" {
		return errors.New("certfile is empty")
	}
	if self.Keyfile == "" {
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
		return server.Serve(listener)
	}
}

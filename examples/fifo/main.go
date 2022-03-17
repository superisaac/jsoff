package main

import (
	"context"
	"flag"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsonz"
	"github.com/superisaac/jsonz/http"
	"net/http"
	"os"
)

func main() {
	flagset := flag.NewFlagSet("jsonz-example-fifo", flag.ExitOnError)
	pBind := flagset.String("bind", "127.0.0.1:6000", "bind address")

	flagset.Parse(os.Args[1:])

	rootCtx := context.Background()

	handler := jsonzhttp.NewSmartHandler(rootCtx, nil, true)

	fifo := make([]interface{}, 0)
	subs := map[string]jsonzhttp.RPCSession{}

	handler.Actor.On("fifo_push", func(req *jsonzhttp.RPCRequest, params []interface{}) (interface{}, error) {
		log.Infof("fifo_push %d items", len(params))
		if len(params) <= 0 {
			return nil, &jsonz.RPCError{Code: -400, Message: "no object is given"}
		}
		fifo = append(fifo, params...)
		for _, elem := range params {
			ntf := jsonz.NewNotifyMessage(
				"fifo_subscription",
				[]interface{}{elem})
			for _, session := range subs {
				log.Infof("push to %s", session.SessionID())
				session.Send(ntf)
			}
		}
		return "ok", nil
	})

	handler.Actor.On("fifo_pop", func(req *jsonzhttp.RPCRequest, params []interface{}) (interface{}, error) {
		log.Infof("fifo_pop")
		if len(fifo) <= 0 {
			return nil, &jsonz.RPCError{Code: -400, Message: "pop empty array"}
		}
		fifo = fifo[:len(fifo)-1]
		return "ok", nil
	})

	handler.Actor.On("fifo_list", func(req *jsonzhttp.RPCRequest, params []interface{}) (interface{}, error) {
		log.Infof("fifo list")
		return fifo, nil
	})

	handler.Actor.OnTyped("fifo_get", func(req *jsonzhttp.RPCRequest, at int) (interface{}, error) {
		log.Infof("fifo get at:%d", at)
		if at < 0 || at >= len(fifo) {
			return nil, &jsonz.RPCError{Code: -400, Message: "index out of range"}
		}
		return fifo[at], nil
	})

	handler.Actor.On("fifo_subscribe", func(req *jsonzhttp.RPCRequest, params []interface{}) (interface{}, error) {
		session := req.Session()
		log.Infof("fifo_subscribe %s", session.SessionID())
		if session == nil {
			return nil, &jsonz.RPCError{Code: -400, Message: "no session established"}
		}
		subs[session.SessionID()] = session
		return "ok", nil
	})

	handler.Actor.OnClose(func(r *http.Request, session jsonzhttp.RPCSession) {
		log.Infof("fifo unsub %s", session.SessionID())
		delete(subs, session.SessionID())
	})

	log.Infof("Example fifo service starts at %s\n", *pBind)
	jsonzhttp.ListenAndServe(rootCtx, *pBind, handler)
}

package main

import (
	"context"
	"flag"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
	"github.com/superisaac/jsoff/http"
	"net/http"
	"os"
	"sync"
)

func main() {
	flagset := flag.NewFlagSet("jsoff-example-fifo", flag.ExitOnError)
	pBind := flagset.String("bind", "127.0.0.1:6000", "bind address")

	flagset.Parse(os.Args[1:])

	rootCtx := context.Background()

	handler := jsoffhttp.NewGatewayHandler(rootCtx, nil, true)

	fifo := make([]interface{}, 0)
	lock := sync.RWMutex{}

	subs := map[string]jsoffhttp.RPCSession{}

	handler.Actor.On("fifo_echo", func(params []interface{}) (interface{}, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return "", nil
		}
	})

	handler.Actor.On("fifo_push", func(params []interface{}) (interface{}, error) {
		lock.Lock()
		defer lock.Unlock()

		log.Infof("fifo_push %d items", len(params))
		if len(params) <= 0 {
			return nil, &jsoff.RPCError{Code: -400, Message: "no object is given"}
		}
		fifo = append(fifo, params...)
		for _, elem := range params {
			ntf := jsoff.NewNotifyMessage(
				"fifo_subscription",
				[]interface{}{elem})
			for _, session := range subs {
				log.Infof("push to %s", session.SessionID())
				session.Send(ntf)
			}
		}
		return "ok", nil
	})

	handler.Actor.On("fifo_pop", func(params []interface{}) (interface{}, error) {
		lock.Lock()
		defer lock.Unlock()

		log.Infof("fifo_pop")
		if len(fifo) <= 0 {
			return nil, &jsoff.RPCError{Code: -400, Message: "pop empty array"}
		}
		fifo = fifo[:len(fifo)-1]
		return "ok", nil
	})

	handler.Actor.On("fifo_list", func(params []interface{}) (interface{}, error) {
		lock.RLock()
		defer lock.RUnlock()

		log.Infof("fifo list")
		return fifo, nil
	})

	handler.Actor.OnTyped("fifo_get", func(at int) (interface{}, error) {
		lock.RLock()
		defer lock.RUnlock()

		log.Infof("fifo get at:%d", at)
		if at < 0 || at >= len(fifo) {
			return nil, &jsoff.RPCError{Code: -400, Message: "index out of range"}
		}
		return fifo[at], nil
	})

	handler.Actor.OnRequest("fifo_subscribe", func(req *jsoffhttp.RPCRequest, params []interface{}) (interface{}, error) {
		session := req.Session()
		if session == nil {
			return "no sesion", nil
		}
		log.Infof("fifo_subscribe %s", session.SessionID())
		if session == nil {
			return nil, &jsoff.RPCError{Code: -400, Message: "no session established"}
		}
		subs[session.SessionID()] = session
		return "ok", nil
	})

	handler.Actor.OnClose(func(r *http.Request, session jsoffhttp.RPCSession) {
		log.Infof("fifo unsub %s", session.SessionID())
		delete(subs, session.SessionID())
	})

	log.Infof("Example fifo service starts at %s\n", *pBind)
	jsoffhttp.ListenAndServe(rootCtx, *pBind, handler)
}

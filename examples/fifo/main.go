package main

import (
	"context"
	"flag"
	log "github.com/sirupsen/logrus"
	"github.com/superisaac/jsoff"
	"github.com/superisaac/jsoff/net"
	"os"
	"sync"
)

func fifoActor() *jsoffnet.Actor {
	fifo := make([]any, 0)
	lock := sync.RWMutex{}

	subs := map[string]jsoffnet.RPCSession{}
	actor := jsoffnet.NewActor()

	actor.On("example_echo", func(params []any) (any, error) {
		if len(params) > 0 {
			return params[0], nil
		} else {
			return "", nil
		}
	})

	actor.On("example_willPanic", func(params []any) (any, error) {
		panic("just panic")
	})

	actor.On("fifo_push", func(params []any) (any, error) {
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
				[]any{elem})
			for _, session := range subs {
				log.Infof("push to %s", session.SessionID())
				session.Send(ntf)
			}
		}
		return "ok", nil
	})

	actor.On("fifo_pop", func(params []any) (any, error) {
		lock.Lock()
		defer lock.Unlock()

		log.Infof("fifo_pop")
		if len(fifo) <= 0 {
			return nil, &jsoff.RPCError{Code: -400, Message: "pop empty array"}
		}
		fifo = fifo[:len(fifo)-1]
		return "ok", nil
	})

	actor.On("fifo_list", func(params []any) (any, error) {
		lock.RLock()
		defer lock.RUnlock()

		log.Infof("fifo list")
		return fifo, nil
	})

	actor.On("fifo_count", func(params []any) (any, error) {
		lock.RLock()
		defer lock.RUnlock()

		log.Infof("fifo count")
		return len(fifo), nil
	}, jsoffnet.WithSchemaJson(`{"description": "get the element count", "params": [], "returns": "integer"}`))

	actor.OnTyped("fifo_get", func(at int) (any, error) {
		lock.RLock()
		defer lock.RUnlock()

		log.Infof("fifo get at:%d", at)
		if at < 0 || at >= len(fifo) {
			return nil, &jsoff.RPCError{Code: -400, Message: "index out of range"}
		}
		return fifo[at], nil
	}, jsoffnet.WithSchemaJson(`{"description": "get an element at index", "type": "method", "params": ["integer"]}`))

	actor.OnRequest("fifo_subscribe", func(req *jsoffnet.RPCRequest, params []any) (any, error) {
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

	actor.OnClose(func(session jsoffnet.RPCSession) {
		log.Infof("fifo unsub %s", session.SessionID())
		delete(subs, session.SessionID())
	})

	return actor
}

func main() {
	flagset := flag.NewFlagSet("jsoff-example-fifo", flag.ExitOnError)
	pTcp := flagset.String("tcp", "", "underline transport, tcp or auto")
	pBind := flagset.String("bind", "", "bind address")

	flagset.Parse(os.Args[1:])

	rootCtx := context.Background()

	actor := fifoActor()

	if *pTcp != "" {
		log.Infof("Example fifo service starts at tcp://%s\n", *pTcp)
		server := jsoffnet.NewTCPServer(rootCtx, actor)
		server.Start(rootCtx, *pTcp)
	} else {
		bind := *pBind
		if bind == "" {
			bind = "127.0.0.1:6000"
		}
		log.Infof("Example fifo service starts at %s\n", bind)
		handler := jsoffnet.NewGatewayHandler(rootCtx, actor, true)
		jsoffnet.ListenAndServe(rootCtx, bind, handler)
	}
}

package rpc

import (	
	"sync"
	"time"
	//"github.com/sniperHW/kendynet"
)

var reqContextPool sync.Pool

var messagePool sync.Pool

func getReqContext(seq uint64,cb RPCResponseHandler) *reqContext {
	var zero time.Time
	c := reqContextPool.Get().(*reqContext)
	c.heapIdx = 0
	c.seq = seq
	c.onResponse = cb
	c.deadline = zero
	return c
}

func putReqContext(c *reqContext) {
	reqContextPool.Put(c)
}

func getMessage() *message {
	m := messagePool.Get().(*message)
	m.data1 = nil
	m.data2 = nil
	return m
}

func putMessage(m *message) {
	messagePool.Put(m)
}

func init() {
	reqContextPool = sync.Pool{} 
	reqContextPool.New = func() interface{} {
		return &reqContext{}
	}

	messagePool = sync.Pool{}
	messagePool.New = func() interface{} {
		return &message{}
 	}
}
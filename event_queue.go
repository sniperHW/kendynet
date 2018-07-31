package kendynet

import (
	"fmt"
	"github.com/sniperHW/kendynet/util"
	"sync/atomic"
)


const (
	tt_noargs  = 1   //无参回调
	tt_varargs = 2   //不定参数回调
)

type element struct {
	tt            int
	args          []interface{}
	callback      interface{}
}

type EventQueue struct {
	eventQueue *util.BlockQueue
	started    int32	
}

func NewEventQueue() *EventQueue {
	r := &EventQueue{}
	r.eventQueue = util.NewBlockQueue()
	return r
}

func (this *EventQueue) Post(callback interface{},args ...interface{}) {
	
	e := element{}

	switch callback.(type) {
	case func():
		e.tt = tt_noargs
		e.callback = callback
		break
	case func([]interface{}):
		e.tt = tt_varargs
		e.callback = callback
		e.args = args
		break
	default:
		panic("invaild callback type")
	}
	this.eventQueue.Add(e)
}

func (this *EventQueue) Close() {
	this.eventQueue.Close()
}

func (this *EventQueue) Run() error {

	if !atomic.CompareAndSwapInt32(&this.started, 0, 1) {
		return fmt.Errorf("already started")
	}

	for {
		closed, localList := this.eventQueue.Get()
		for _,v := range(localList) {
			e := v.(element)
			if e.tt == tt_noargs {
				e.callback.(func())()
			} else {
				e.callback.(func([]interface{}))(e.args)
			}
		}
		if closed {
			return nil
		}
	}
}
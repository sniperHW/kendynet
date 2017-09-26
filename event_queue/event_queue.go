package event_queue

import (
	"github.com/sniperHW/kendynet/util"
	"sync/atomic"
	"fmt"
)

type EventQueue struct {
	eventQueue *util.BlockQueue
	started     int32
}

func New() *EventQueue {
	r := &EventQueue{}
	r.eventQueue = util.NewBlockQueue()
	return r
}

func (this *EventQueue) PostEvent(ev interface{}) error {
	return this.eventQueue.Add(ev)
}

func (this *EventQueue) Close() {
	this.eventQueue.Close()
}

func (this *EventQueue) Start(onEvent func(interface{})) error {
	
	if nil == onEvent {
		return fmt.Errorf("onEvent == nil")
	}

	if !atomic.CompareAndSwapInt32(&this.started,0,1) {
        return fmt.Errorf("started")
    }

	localList := util.NewList()

	for {

		closed := this.eventQueue.Get(localList)

		if closed {
			return nil
		}

		for !localList.Empty() {
			ev := localList.Pop()
			onEvent(ev)
		}
	}
}

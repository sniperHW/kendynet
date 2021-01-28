package event

import (
	//"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"reflect"
	"sync"
	"sync/atomic"
)

type Handle *handle

type handList struct {
	head handle
	tail handle
}

type handle struct {
	pprev   *handle
	nnext   *handle
	fn      interface{}
	once    bool
	event   interface{}
	removed int32
	version int64
}

const (
	opEmit     = 1
	opDelete   = 2
	opRegister = 3
)

type op struct {
	opType int
	args   []interface{}
	h      *handle
}

type handlerSlot struct {
	sync.Mutex
	l         handList
	emiting   bool
	current   int
	version   int64
	pendingOP []op
}

type EventHandler struct {
	sync.RWMutex
	slots        map[interface{}]*handlerSlot
	processQueue *EventQueue
	version      int64
}

func NewEventHandler(processQueue ...*EventQueue) *EventHandler {
	var q *EventQueue
	if len(processQueue) > 0 {
		q = processQueue[0]
	}
	return &EventHandler{
		slots:        map[interface{}]*handlerSlot{},
		processQueue: q,
	}
}

func (this *EventHandler) register(event interface{}, once bool, fn interface{}) Handle {
	if nil == event {
		panic("event == nil")
	}

	if reflect.TypeOf(fn).Kind() != reflect.Func {
		panic("fn should be func type")

	}

	h := &handle{
		fn:    fn,
		once:  once,
		event: event,
	}

	this.Lock()
	defer this.Unlock()
	slot, ok := this.slots[event]
	if !ok {
		slot = &handlerSlot{
			version: atomic.AddInt64(&this.version, 1),
		}

		slot.l.head.nnext = &slot.l.tail
		slot.l.tail.pprev = &slot.l.head

		this.slots[h.event] = slot
	}
	slot.register(h)

	return Handle(h)
}

func (this *EventHandler) RegisterOnce(event interface{}, fn interface{}) Handle {
	return this.register(event, true, fn)
}

func (this *EventHandler) Register(event interface{}, fn interface{}) Handle {
	return this.register(event, false, fn)
}

func (this *EventHandler) Remove(h Handle) {
	hh := (*handle)(h)
	this.RLock()
	slot, ok := this.slots[hh.event]
	this.RUnlock()
	if ok && hh.version == slot.version {
		slot.remove(h)
	}
}

//触发事件
func (this *EventHandler) Emit(event interface{}, args ...interface{}) {
	this.RLock()
	slot, ok := this.slots[event]
	this.RUnlock()
	if ok {
		if this.processQueue != nil {
			this.processQueue.PostNoWait(0, slot.emit, args...)
		} else {
			slot.emit(args...)
		}
	}
}

func (this *EventHandler) EmitWithPriority(priority int, event interface{}, args ...interface{}) {
	this.RLock()
	slot, ok := this.slots[event]
	this.RUnlock()
	if ok {
		if this.processQueue != nil {
			this.processQueue.PostNoWait(priority, slot.emit, args...)
		} else {
			slot.emit(args...)
		}
	}
}

func (this *EventHandler) Clear(event interface{}) {
	this.Lock()
	defer this.Unlock()
	delete(this.slots, event)
}

func (this *handlerSlot) doRegister(h *handle) {
	h.version = this.version
	this.l.tail.pprev.nnext = h
	h.nnext = &this.l.tail
	h.pprev = this.l.tail.pprev
	this.l.tail.pprev = h
}

func (this *handlerSlot) register(h *handle) {
	this.Lock()
	defer this.Unlock()
	if this.emiting {
		this.pendingOP = append(this.pendingOP, op{
			opType: opRegister,
			h:      h,
		})
	} else {
		this.doRegister(h)
	}
}

func (this *handlerSlot) doRemove(h *handle) {
	h.pprev.nnext = h.nnext
	h.nnext.pprev = h.pprev
	h.version = 0
}

func (this *handlerSlot) remove(h *handle) {
	this.Lock()
	defer this.Unlock()
	if this.emiting {
		this.pendingOP = append(this.pendingOP, op{
			opType: opDelete,
			h:      h,
		})
	} else {
		this.doRemove(h)
	}
}

func pcall2(h *handle, args []interface{}) {

	var arguments []interface{}

	fnType := reflect.TypeOf(h.fn)

	if fnType.NumIn() > 0 && fnType.In(0) == reflect.TypeOf((Handle)(h)) {
		arguments = make([]interface{}, len(args)+1, len(args)+1)
		arguments[0] = h
		copy(arguments[1:], args)
	} else {
		arguments = args
	}

	if _, err := util.ProtectCall(h.fn, arguments...); err != nil {
		logger := kendynet.GetLogger()
		if logger != nil {
			logger.Errorln(err)
		}
	}
}

func (this *handlerSlot) doEmit(args []interface{}) {
	pre := &this.l.head
	cur := this.l.head.nnext
	for cur != &this.l.tail {
		pcall2(cur, args)
		if cur.once {
			pre.nnext = cur.nnext
			cur.nnext = nil
			cur.version = 0
			cur = pre.nnext
		} else {
			pre = cur
			cur = cur.nnext
		}
	}
}

func (this *handlerSlot) emit(args ...interface{}) {
	this.Lock()
	this.pendingOP = append(this.pendingOP, op{
		opType: opEmit,
		args:   args,
	})

	if this.emiting {
		this.Unlock()
	} else {
		this.emiting = true
		size := len(this.pendingOP)
		for i := 0; i < size; i++ {
			op := this.pendingOP[i]
			switch op.opType {
			case opDelete:
				this.doRemove(op.h)
			case opRegister:
				this.doRegister(op.h)
			case opEmit:
				this.Unlock()
				this.doEmit(op.args)
				this.Lock()
			}
			size = len(this.pendingOP)
		}
		this.emiting = false
		this.pendingOP = []op{}
		this.Unlock()
	}
}

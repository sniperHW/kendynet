package goroutine

import (
	"github.com/sniperHW/kendynet/util"
	"sync"
	"sync/atomic"
)

var ReserveCount int = 1000
var MaxCount int = 10000

type task struct {
	nnext *task
	args  []interface{}
	fn    interface{}
}

type routine struct {
	nnext  *routine
	notify chan struct{}
}

type pool struct {
	mu               sync.Mutex
	ttail            *task
	waittail         *routine
	routineCount     int
	totalCreateCount int64
	onError          atomic.Value
	option           Option
}

type Option struct {
	ReserveCount    int //保留的goroutine数量
	MaxCount        int //最大允许的goroutine数量
	MaxCurrentCount int //最大并发数量，值大于0为开启
}

func New(o Option) *pool {
	p := &pool{
		option: o,
	}
	return p
}

var defaultPool *pool = New(Option{
	ReserveCount: ReserveCount,
	MaxCount:     MaxCount,
})

func (this *pool) wait(p *routine) {
	var head *routine
	if this.waittail == nil {
		head = p
	} else {
		head = this.waittail.nnext
		this.waittail.nnext = p
	}
	p.nnext = head
	this.waittail = p
	<-p.notify
}

func (this *pool) singal() {
	if this.waittail != nil {
		head := this.waittail.nnext
		if head == this.waittail {
			this.waittail = nil
		} else {
			this.waittail.nnext = head.nnext
		}
		head.nnext = nil
		select {
		case head.notify <- struct{}{}:
		default:
		}
	} else {
		panic("panic here")
	}
}

func (this *pool) push(t *task) (create bool) {
	var head *task
	if this.ttail == nil {
		head = t
	} else {
		head = this.ttail.nnext
		this.ttail.nnext = t
	}
	t.nnext = head
	this.ttail = t

	if this.waittail != nil {
		this.singal()
	} else if this.routineCount < this.option.MaxCount {
		this.routineCount++
		this.totalCreateCount++
		create = true
	}

	return
}

func (this *pool) pop(r *routine) *task {
	this.mu.Lock()
	for this.ttail == nil {
		this.wait(r)
	}
	head := this.ttail.nnext
	if head == this.ttail {
		this.ttail = nil
	} else {
		this.ttail.nnext = head.nnext
	}
	this.mu.Unlock()
	return head
}

func (this *pool) OnError(onError func(error)) {
	this.onError.Store(onError)
}

func (this *pool) createNewRoutine() {
	go func() {
		r := &routine{
			notify: make(chan struct{}),
		}
		for {
			t := this.pop(r)
			_, err := util.ProtectCall(t.fn, t.args...)
			if nil != err {
				onError := this.onError.Load()
				if nil == onError {
					panic(err.Error())
				} else {
					onError.(func(error))(err)
				}
			}

			shouldBreak := false
			this.mu.Lock()
			if this.routineCount > this.option.ReserveCount {
				this.routineCount--
				shouldBreak = true
			}
			this.mu.Unlock()
			if shouldBreak {
				break
			}
		}
	}()
}

func (this *pool) Go(fn interface{}, args ...interface{}) {
	var create bool
	this.mu.Lock()
	create = this.push(&task{
		fn:   fn,
		args: args,
	})
	this.mu.Unlock()
	if create {
		this.createNewRoutine()
	}
}

func OnError(onError func(error)) {
	defaultPool.OnError(onError)
}

func Go(fn interface{}, args ...interface{}) {
	defaultPool.Go(fn, args...)
}

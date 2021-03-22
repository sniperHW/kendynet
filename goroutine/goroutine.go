package goroutine

import (
	"github.com/sniperHW/kendynet/util"
	"sync"
	"sync/atomic"
)

var ReserveCount int = 1000
var MaxCount int = 10000

type Option struct {
	ReserveCount    int //保留的goroutine数量
	MaxCount        int //最大允许的goroutine数量
	MaxCurrentCount int //最大并发数量，值大于0为开启
}

type task struct {
	nnext *task
	args  []interface{}
	fn    interface{}
}

type routine struct {
	nnext *routine
	cond  *sync.Cond
}

type pool struct {
	mu               sync.Mutex
	ttail            *task
	taskcount        int
	waittail         *routine
	waitcount        int
	routineCount     int
	totalCreateCount int64
	onError          atomic.Value
	option           Option
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

func (this *pool) wait(r *routine) {
	var head *routine
	if this.waittail == nil {
		head = r
	} else {
		head = this.waittail.nnext
		this.waittail.nnext = r
	}
	r.nnext = head
	this.waittail = r
	this.waitcount++
	r.cond.Wait()
}

func (this *pool) push(t *task) (create bool, r *routine) {
	var head *task
	if this.ttail == nil {
		head = t
	} else {
		head = this.ttail.nnext
		this.ttail.nnext = t
	}
	t.nnext = head
	this.ttail = t
	this.taskcount++

	if this.waittail != nil {
		r = this.waittail.nnext
		if r == this.waittail {
			this.waittail = nil
		} else {
			this.waittail.nnext = r.nnext
		}
		r.nnext = nil
		this.waitcount--
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
	this.taskcount--
	this.mu.Unlock()
	return head
}

func (this *pool) OnError(onError func(error)) {
	this.onError.Store(onError)
}

func (this *pool) createNewRoutine() {
	go func() {
		r := &routine{
			cond: sync.NewCond(&this.mu),
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

//如果超过最大并发限制返回false
func (this *pool) Go(fn interface{}, args ...interface{}) bool {
	this.mu.Lock()
	if this.option.MaxCurrentCount > 0 {
		workcount := this.routineCount - this.waitcount
		if this.routineCount >= this.option.MaxCount && workcount+this.taskcount > this.option.MaxCurrentCount {
			this.mu.Unlock()
			return false
		}
	}
	var create bool
	var r *routine
	create, r = this.push(&task{
		fn:   fn,
		args: args,
	})
	this.mu.Unlock()
	if create {
		this.createNewRoutine()
	} else if nil != r {
		r.cond.Signal()
	}
	return true
}

func OnError(onError func(error)) {
	defaultPool.OnError(onError)
}

func Go(fn interface{}, args ...interface{}) bool {
	return defaultPool.Go(fn, args...)
}

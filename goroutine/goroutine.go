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

type pool struct {
	mu               sync.Mutex
	cond             *sync.Cond
	tail             *task
	waitCount        int
	routineCount     int
	totalCreateCount int64
	onError          atomic.Value
}

var defaultPool *pool = func() *pool {
	p := &pool{}
	p.cond = sync.NewCond(&p.mu)
	return p
}()

func (this *pool) push(t *task) {
	this.mu.Lock()
	var head *task
	if this.tail == nil {
		head = t
	} else {
		head = this.tail.nnext
		this.tail.nnext = t
	}
	t.nnext = head
	this.tail = t

	waitCount := this.waitCount
	create := false
	if waitCount == 0 && this.routineCount < MaxCount {
		this.routineCount++
		this.totalCreateCount++
		create = true
	}
	this.mu.Unlock()

	if create {
		this.createNewRoutine()
	} else if waitCount > 0 {
		this.cond.Signal()
	}
}

func (this *pool) pop() *task {
	this.mu.Lock()
	for this.tail == nil {
		this.waitCount++
		this.cond.Wait()
		this.waitCount--
	}
	head := this.tail.nnext
	if head == this.tail {
		this.tail = nil
	} else {
		this.tail.nnext = head.nnext
	}
	this.mu.Unlock()
	return head
}

func (this *pool) OnError(onError func(error)) {
	this.onError.Store(onError)
}

func (this *pool) createNewRoutine() {
	go func() {
		for {
			t := this.pop()
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
			if this.routineCount > ReserveCount {
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
	this.push(&task{
		fn:   fn,
		args: args,
	})
}

func OnError(onError func(error)) {
	defaultPool.OnError(onError)
}

func Go(fn interface{}, args ...interface{}) {
	defaultPool.Go(fn, args...)
}

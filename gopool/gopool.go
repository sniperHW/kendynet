package gopool

import (
	"errors"
	"sync"
)

type queItem struct {
	nnext *queItem
	pprev *queItem
	v     interface{}
}

var queItemPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		return &queItem{}
	},
}

func getQueItem() *queItem {
	return queItemPool.Get().(*queItem)
}

func putQueItem(i *queItem) {
	i.v = nil
	queItemPool.Put(i)
}

type que struct {
	cap  int
	size int
	head queItem
}

func (q *que) push(v interface{}) bool {
	if q.size < q.cap {
		n := getQueItem()
		n.v = v
		tail := q.head.pprev

		n.nnext = tail.nnext
		n.pprev = tail

		tail.nnext = n
		q.head.pprev = n

		q.size++
		return true
	} else {
		return false
	}
}

func (q *que) pop() interface{} {
	if q.head.nnext == &q.head {
		return nil
	} else {
		first := q.head.nnext
		v := first.v
		q.remove(first)
		putQueItem(first)
		q.size--
		return v
	}
}

func (q *que) remove(n *queItem) {
	if nil != n.nnext && nil != n.pprev && n.nnext != n && n.pprev != n {
		next := n.nnext
		prev := n.pprev
		prev.nnext = next
		next.pprev = prev
		n.nnext = nil
		n.pprev = nil
	}
}

type task struct {
	f interface{}
	v interface{}
}

type routine struct {
	taskCh chan task
}

func (r *routine) run(p *Pool) {
	for task := range r.taskCh {
		switch task.f.(type) {
		case func():
			task.f.(func())()
		case func(...interface{}):
			task.f.(func(...interface{}))(task.v.([]interface{})...)
		default:
			panic("invaild element")
		}
		p.free(r)
	}
}

var defaultPool *Pool = New(Option{
	MaxRoutineCount: 1000,
	Mode:            QueueMode,
	MaxQueueSize:    1024,
})

type Mode int

const (
	QueueMode = Mode(0) //队列模式，如果达到goroutine上限且没有空闲goroutine,将任务置入队列
	GoMode    = Mode(1) //如果达到goroutine上限且没有空闲goroutine,开启单独的goroutine执行
)

type Option struct {
	MaxRoutineCount int //最大goroutine数量
	MaxQueueSize    int //最大排队任务数量
	Mode            Mode
}

type Pool struct {
	sync.Mutex
	frees   que
	taskQue que
	count   int
	o       Option
}

func New(o Option) *Pool {
	switch o.Mode {
	case QueueMode, GoMode:
	default:
		return nil
	}

	if o.MaxRoutineCount == 0 {
		o.MaxRoutineCount = 1024
	}

	p := &Pool{
		o:     o,
		frees: que{cap: o.MaxRoutineCount},
	}

	p.frees.head.nnext = &p.frees.head
	p.frees.head.pprev = &p.frees.head

	if o.Mode == QueueMode {
		if o.MaxQueueSize == 0 {
			o.MaxQueueSize = 1024
		}
		p.taskQue.cap = o.MaxQueueSize

		p.taskQue.head.nnext = &p.taskQue.head
		p.taskQue.head.pprev = &p.taskQue.head
	}

	return p
}

func (p *Pool) free(r *routine) {
	p.Lock()
	defer p.Unlock()
	switch p.o.Mode {
	case QueueMode:
		if v := p.taskQue.pop(); nil != v {
			r.taskCh <- v.(task)
			return
		}
	}

	p.frees.push(r)
}

func (p *Pool) popFree() *routine {
	if v := p.frees.pop(); nil != v {
		return v.(*routine)
	} else {
		return nil
	}
}

func (p *Pool) gogo(f interface{}, v []interface{}) error {
	p.Lock()
	defer p.Unlock()
	r := p.popFree()
	if nil != r {
		r.taskCh <- task{f: f, v: v}
	} else {
		if p.count == p.o.MaxRoutineCount {
			switch p.o.Mode {
			case GoMode:
				switch f.(type) {
				case func():
					go f.(func())()
				case func(...interface{}):
					go f.(func(...interface{}))(v...)
				default:
					return errors.New("invaild arg")
				}
			case QueueMode:
				if !p.taskQue.push(task{f: f, v: v}) {
					return errors.New("exceed MaxQueueSize")
				}
			}
		} else {
			p.count++
			r = &routine{
				taskCh: make(chan task, 1),
			}
			r.taskCh <- task{f: f, v: v}
			go r.run(p)
		}
	}
	return nil
}

func (p *Pool) Go(f func()) error {
	return p.gogo(f, nil)
}

func (p *Pool) GoWithParams(f func(...interface{}), v ...interface{}) error {
	return p.gogo(f, v)
}

func Go(f func()) error {
	return defaultPool.Go(f)
}

func GoWithParams(f func(...interface{}), v ...interface{}) error {
	return defaultPool.GoWithParams(f, v...)
}

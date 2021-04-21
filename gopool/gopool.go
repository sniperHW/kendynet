package gopool

import (
	"container/list"
	"errors"
	"sync"
)

type routine struct {
	taskCh chan func()
}

func (r *routine) run(p *pool) {
	for task := range r.taskCh {
		task()
		p.free(r)
	}
}

var defaultPool *pool = New(Option{
	MaxRoutineCount: 1000,
	Mode:            QueueMode,
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

type pool struct {
	sync.Mutex
	head  int
	tail  int
	frees []*routine
	queue *list.List
	count int
	o     Option
}

func New(o Option) *pool {
	switch o.Mode {
	case QueueMode, GoMode:
	default:
		return nil
	}

	if o.MaxRoutineCount == 0 {
		o.MaxRoutineCount = 100
	}

	p := &pool{
		o:     o,
		frees: make([]*routine, o.MaxRoutineCount+1, o.MaxRoutineCount+1),
	}

	if o.Mode == QueueMode {
		p.queue = list.New()
	}

	return p
}

func (p *pool) free(r *routine) {
	p.Lock()
	defer p.Unlock()
	switch p.o.Mode {
	case QueueMode:
		f := p.queue.Front()
		if nil != f {
			r.taskCh <- f.Value.(func())
			p.queue.Remove(f)
			return
		}
	}

	p.frees[p.tail] = r
	p.tail = (p.tail + 1) % len(p.frees)

}

func (p *pool) popFree() *routine {
	if p.head != p.tail {
		head := p.frees[p.head]
		p.frees[p.head] = nil
		p.head = (p.head + 1) % len(p.frees)
		return head
	} else {
		return nil
	}
}

func (p *pool) Go(f func()) error {
	p.Lock()
	defer p.Unlock()
	r := p.popFree()
	if nil != r {
		r.taskCh <- f
	} else {
		if p.count == p.o.MaxRoutineCount {
			switch p.o.Mode {
			case GoMode:
				go f()
			case QueueMode:
				if p.o.MaxQueueSize > 0 && p.queue.Len() == p.o.MaxQueueSize {
					return errors.New("exceed MaxQueueSize")
				}
				p.queue.PushBack(f)
			}
		} else {
			p.count++
			r = &routine{
				taskCh: make(chan func(), 1),
			}
			r.taskCh <- f
			go r.run(p)
		}
	}
	return nil
}

func Go(f func()) error {
	return defaultPool.Go(f)
}

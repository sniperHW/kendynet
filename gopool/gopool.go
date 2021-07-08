package gopool

import (
	"errors"
	"sync"
)

var (
	Err_PoolClosed    = errors.New("pool closed")
	Err_TaskQueueFull = errors.New("task queue full")
)

type routine struct {
	nnext  *routine
	taskCh chan func()
}

func (r *routine) run(p *Pool) {
	var ok bool
	for task := range r.taskCh {
		task()
		for {
			ok, task = p.putRoutine(r)
			if !ok {
				return
			} else if nil != task {
				task()
			} else {
				break
			}
		}
	}
}

type Option struct {
	/*
	 * 最大goroutine数量(<=0无限制),允许同时存在的goroutine数量上限
	 */
	MaxRoutineCount int

	/*
	 * 保留的goroutine数量,创建的routine在执行完task且任务队列为空时，判断当前goroutine数量是否超过ReserveRoutineCount,如果是routine退出
	 * 否则放回空闲routine队列。
	 *
	 * 如果MaxRoutineCount没设置，但是设置了ReserveRoutineCount,如果当前goroutine数量已经达到ReserveRoutineCount,且没有空闲的goroutine可用
	 * 执行Pool.Go的时候不会创建routine对象而是直接调用go task()执行任务。
	 */

	ReserveRoutineCount int

	/*
	 * 最大排队任务数量(<0无限制),如果设置了MaxRoutineCount,且当前没有空闲的goroutine可用，执行Pool.Go时把task push进任务队列
	 */
	MaxQueueSize int
}

const maxCacheItemCount = 4096

var gItemPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		return &listItem{}
	},
}

type listItem struct {
	nnext *listItem
	v     func()
}

type linkList struct {
	tail      *listItem
	count     int
	freeItems *listItem
	freeCount int
}

func (this *linkList) pushItem(l **listItem, item *listItem) {
	var head *listItem
	if *l == nil {
		head = item
	} else {
		head = (*l).nnext
		(*l).nnext = item
	}
	item.nnext = head
	*l = item
}

func (this *linkList) popItem(l **listItem) *listItem {
	if *l == nil {
		return nil
	} else {
		item := (*l).nnext
		if item == (*l) {
			(*l) = nil
		} else {
			(*l).nnext = item.nnext
		}

		item.nnext = nil
		return item
	}
}

func (this *linkList) getPoolItem(v func()) *listItem {
	/*
	 * 本地的cache中获取listItem,如果没有才去sync.Pool取
	 */
	item := this.popItem(&this.freeItems)
	if nil == item {
		item = gItemPool.Get().(*listItem)
	} else {
		this.freeCount--
	}
	item.v = v
	return item
}

func (this *linkList) putPoolItem(item *listItem) {
	item.v = nil
	if this.freeCount < maxCacheItemCount {
		//如果尚未超过本地cache限制，将listItem放回本地cache供下次使用
		this.freeCount++
		this.pushItem(&this.freeItems, item)
	} else {
		gItemPool.Put(item)
	}
}

func (this *linkList) push(v func()) {
	this.pushItem(&this.tail, this.getPoolItem(v))
	this.count++
}

func (this *linkList) pop() func() {
	item := this.popItem(&this.tail)
	if nil == item {
		return nil
	} else {
		this.count--
		v := item.v
		this.putPoolItem(item)
		return v
	}
}

var defaultPool *Pool = New(Option{
	ReserveRoutineCount: 1024,
})

type Pool struct {
	sync.Mutex
	die          bool
	routineCount int
	o            Option
	freeRoutines *routine
	taskQueue    linkList
}

func New(o Option) *Pool {
	return &Pool{
		o: o,
	}
}

func (p *Pool) putRoutine(r *routine) (bool, func()) {
	p.Lock()
	if p.die {
		p.Unlock()
		return false, nil
	} else {
		v := p.taskQueue.pop()
		if nil != v {
			p.Unlock()
			return true, v
		} else {
			if p.routineCount > p.o.ReserveRoutineCount {
				p.routineCount--
				p.Unlock()
				return false, nil
			} else {
				var head *routine
				if p.freeRoutines == nil {
					head = r
				} else {
					head = p.freeRoutines.nnext
					p.freeRoutines.nnext = r
				}
				r.nnext = head
				p.freeRoutines = r
				p.Unlock()
				return true, nil
			}
		}
	}
}

func (p *Pool) getRoutine() *routine {
	if p.freeRoutines == nil {
		return nil
	} else {
		r := p.freeRoutines.nnext
		if r == p.freeRoutines {
			p.freeRoutines = nil
		} else {
			p.freeRoutines.nnext = r.nnext
		}

		r.nnext = nil
		return r
	}
}

func (p *Pool) Go(task func()) (err error) {
	p.Lock()
	if p.die {
		p.Unlock()
		err = Err_PoolClosed
	} else if r := p.getRoutine(); nil != r {
		p.Unlock()
		r.taskCh <- task
	} else {
		if p.o.MaxRoutineCount > 0 && p.routineCount == p.o.MaxRoutineCount {
			if p.o.MaxQueueSize >= 0 && p.taskQueue.count >= p.o.MaxQueueSize {
				err = Err_TaskQueueFull
			} else {
				p.taskQueue.push(task)
			}
			p.Unlock()
		} else if p.o.MaxRoutineCount <= 0 && p.routineCount >= p.o.ReserveRoutineCount {
			p.Unlock()
			go task()
		} else {
			p.routineCount++
			p.Unlock()
			r := &routine{taskCh: make(chan func())}
			go r.run(p)
			r.taskCh <- task
		}
	}
	return
}

func (p *Pool) Close() {
	p.Lock()
	defer p.Unlock()
	if !p.die {
		p.die = true
		for r := p.getRoutine(); nil != r; r = p.getRoutine() {
			close(r.taskCh)
		}
	}
}

func Go(f func()) error {
	return defaultPool.Go(f)
}

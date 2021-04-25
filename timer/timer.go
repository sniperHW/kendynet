package timer

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"sync"
	"sync/atomic"
	"time"
)

const (
	waitting int32 = 0
	firing   int32 = 1
	removed  int32 = 2
)

type indexToTimer struct {
	sync.Mutex
	m map[uint64]*Timer
}

func (this *indexToTimer) add(t *Timer) bool {
	this.Lock()
	defer this.Unlock()
	if _, ok := this.m[*t.index]; ok {
		return false
	} else {
		this.m[*t.index] = t
		return true
	}
}

func (this *indexToTimer) get(index uint64) *Timer {
	this.Lock()
	defer this.Unlock()
	return this.m[index]
}

func (this *indexToTimer) delete(index uint64) *Timer {
	this.Lock()
	defer this.Unlock()
	if t, ok := this.m[index]; ok {
		delete(this.m, index)
		return t
	} else {
		return nil
	}
}

type IndexMgr struct {
	indexToTimer [63]*indexToTimer
}

func NewIndexMgr() IndexMgr {
	m := IndexMgr{}
	for k, _ := range m.indexToTimer {
		m.indexToTimer[k] = &indexToTimer{
			m: map[uint64]*Timer{},
		}
	}
	return m
}

var defaultIndexMgr IndexMgr = NewIndexMgr()

type Timer struct {
	duration time.Duration
	status   int32
	callback func(*Timer, interface{})
	t        atomic.Value
	ud       interface{}
	index    *uint64
	repeat   bool
	mgr      *indexToTimer
}

func (this *Timer) call() {
	if atomic.CompareAndSwapInt32(&this.status, waitting, firing) {
		if _, err := util.ProtectCall(this.callback, this, this.ud); nil != err {
			if logger := kendynet.GetLogger(); nil != logger {
				logger.Error("error on timer:", err.Error())
			}
		}
		if this.repeat {
			if atomic.CompareAndSwapInt32(&this.status, firing, waitting) {
				//1
				this.t.Store(time.AfterFunc(this.duration, func() {
					this.call()
				}))
				/*
				 * 执行到1的时候,其它线程可能会调用remove,新的定时器还没被设置，因此在remove中Stop的是旧的定时器
				 * 因此这里需要再次判断是否执行了removed,如果是则将前面设置的定时器Stop
				 */
				if atomic.LoadInt32(&this.status) == removed {
					this.t.Load().(*time.Timer).Stop()
				}
			}
		} else {
			atomic.StoreInt32(&this.status, removed)
			if this.index != nil {
				this.mgr.delete(*this.index)
			}
		}
	}

}

func (this *Timer) Cancel() bool {
	if atomic.CompareAndSwapInt32(&this.status, waitting, removed) {
		this.t.Load().(*time.Timer).Stop()
		if nil != this.index {
			this.mgr.delete(*this.index)
		}
		return true
	} else {
		atomic.StoreInt32(&this.status, removed)
		return false
	}
}

//只对一次性定时器有效
func (this *Timer) ResetFireTime(timeout time.Duration) bool {
	if this.repeat || atomic.LoadInt32(&this.status) != waitting {
		return false
	}
	return this.t.Load().(*time.Timer).Reset(timeout)
}

func (this *Timer) GetCTX() interface{} {
	return this.ud
}

func newTimer(mgr *indexToTimer, timeout time.Duration, repeat bool, fn func(*Timer, interface{}), ud interface{}, index *uint64) *Timer {
	if nil != fn {
		t := &Timer{
			duration: timeout,
			callback: fn,
			ud:       ud,
			repeat:   repeat,
			index:    index,
			mgr:      mgr,
		}

		t.t.Store(time.AfterFunc(t.duration, func() {
			t.call()
		}))

		if nil != index {
			if !mgr.add(t) {
				return nil
			}
		}

		return t

	} else {
		return nil
	}
}

func (this *IndexMgr) GetTimerByIndex(index uint64) *Timer {
	return this.indexToTimer[index%uint64(len(this.indexToTimer))].get(index)
}

func (this *IndexMgr) OnceWithIndex(timeout time.Duration, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	return newTimer(this.indexToTimer[index%uint64(len(this.indexToTimer))], timeout, false, callback, ctx, &index)
}

func (this *IndexMgr) CancelByIndex(index uint64) (bool, interface{}) {
	if t := this.indexToTimer[index%uint64(len(this.indexToTimer))].delete(index); nil != t {
		if atomic.CompareAndSwapInt32(&t.status, waitting, removed) {
			return true, t.ud
		} else {
			atomic.StoreInt32(&t.status, removed)
		}
	}
	return false, nil
}

//一次性定时器
func Once(timeout time.Duration, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return newTimer(nil, timeout, false, callback, ctx, nil)
}

//重复定时器
func Repeat(duration time.Duration, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return newTimer(nil, duration, true, callback, ctx, nil)
}

func OnceWithIndex(timeout time.Duration, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	return defaultIndexMgr.OnceWithIndex(timeout, callback, ctx, index)
}

func GetTimerByIndex(index uint64) *Timer {
	return defaultIndexMgr.GetTimerByIndex(index)
}

func CancelByIndex(index uint64) (bool, interface{}) {
	return defaultIndexMgr.CancelByIndex(index)
}

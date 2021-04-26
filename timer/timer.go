package timer

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"sync/atomic"
	"time"
)

const (
	waitting int32 = 0
	firing   int32 = 1
	removed  int32 = 2
)

type Timer struct {
	duration time.Duration
	status   int32
	callback func(*Timer, interface{})
	t        atomic.Value
	ud       interface{}
	repeat   bool
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
		}
	}

}

func (this *Timer) Cancel() bool {
	if atomic.CompareAndSwapInt32(&this.status, waitting, removed) {
		this.t.Load().(*time.Timer).Stop()
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

func newTimer(timeout time.Duration, repeat bool, fn func(*Timer, interface{}), ud interface{}) *Timer {
	if nil != fn {
		t := &Timer{
			duration: timeout,
			callback: fn,
			ud:       ud,
			repeat:   repeat,
		}

		t.t.Store(time.AfterFunc(t.duration, func() {
			t.call()
		}))

		return t

	} else {
		return nil
	}
}

//一次性定时器
func Once(timeout time.Duration, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return newTimer(timeout, false, callback, ctx)
}

//重复定时器
func Repeat(duration time.Duration, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return newTimer(duration, true, callback, ctx)
}

/*package timer

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/util"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	globalMgr *TimerMgr = NewTimerMgr()
)

const (
	waitting int32 = 0
	firing   int32 = 1
	removed  int32 = 2
)

type Timer struct {
	eventQue *event.EventQueue
	duration time.Duration
	repeat   bool //是否重复定时器
	status   int32
	callback func(*Timer, interface{})
	p        *p
	ctx      interface{}
	index    uint64
	t        atomic.Value
}

type p struct {
	sync.Mutex
	index2Timer map[uint64]*Timer
}

func (this *Timer) GetCTX() interface{} {
	return this.ctx
}

func (this *Timer) call_() {
	if atomic.CompareAndSwapInt32(&this.status, waitting, firing) {
		if _, err := util.ProtectCall(this.callback, this, this.ctx); nil != err {
			logger := kendynet.GetLogger()
			if nil != logger {
				logger.Errorln("error on timer:", err.Error())
			} else {
				fmt.Println("error on timer:", err.Error())
			}
		}

		if this.repeat {
			this.p.resetTicker(this)
		} else {
			atomic.StoreInt32(&this.status, removed)
			if this.index != 0 {
				this.p.Lock()
				delete(this.p.index2Timer, this.index)
				this.p.Unlock()
			}
		}
	}
}

func (this *Timer) call() {
	if nil == this.eventQue {
		this.call_()
	} else {
		this.eventQue.PostNoWait(func() {
			this.call_()
		})
	}
}

func newp() *p {
	mgr := &p{
		index2Timer: map[uint64]*Timer{},
	}
	return mgr
}

/*
 *  timeout:    超时时间
 *  repeat:     是否重复定时器
 *  eventQue:   如果非nil,callback会被投递到eventQue，否则在定时器主循环中执行
 * /

func (this *p) newTimer(timeout time.Duration, repeat bool, eventQue *event.EventQueue, fn func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	if nil != fn {
		t := &Timer{
			duration: timeout,
			repeat:   repeat,
			callback: fn,
			eventQue: eventQue,
			p:        this,
			ctx:      ctx,
			index:    index,
		}
		if this.addTimer(t, index) {
			return t
		} else {
			return nil
		}
	} else {
		return nil
	}
}

func (this *p) addTimer(t *Timer, index uint64) bool {
	if index > 0 {
		this.Lock()
		defer this.Unlock()
		if _, ok := this.index2Timer[index]; ok {
			return false
		} else {
			this.index2Timer[index] = t
			t.t.Store(time.AfterFunc(t.duration, func() {
				t.call()
			}))
		}
	} else {
		t.t.Store(time.AfterFunc(t.duration, func() {
			t.call()
		}))
	}
	return true
}

func (this *p) GetTimerByIndex(index uint64) *Timer {
	this.Lock()
	defer this.Unlock()
	if t, ok := this.index2Timer[index]; ok {
		return t
	} else {
		return nil
	}
}

func (this *p) resetTicker(t *Timer) {
	if atomic.CompareAndSwapInt32(&t.status, firing, waitting) {
		duration := time.Duration(atomic.LoadInt64((*int64)(&t.duration)))
		t.t.Store(time.AfterFunc(duration, func() {
			t.call()
		}))
		if atomic.LoadInt32(&t.status) == removed {
			t.t.Load().(*time.Timer).Stop()
		}
	}
}

func (this *p) resetFireTime(t *Timer, timeout time.Duration) bool {
	if t.repeat || atomic.LoadInt32(&t.status) != waitting {
		return false
	}
	return t.t.Load().(*time.Timer).Reset(timeout)
}

func (this *p) resetDuration(t *Timer, duration time.Duration) bool {
	if !t.repeat {
		return false
	} else {
		atomic.StoreInt64((*int64)(&t.duration), int64(duration))
		for {
			if atomic.LoadInt32(&t.status) == removed {
				return false
			} else {
				if t.t.Load().(*time.Timer).Reset(duration) {
					break
				}
			}
		}
		return true
	}
}

func (this *p) remove(t *Timer) bool {
	if atomic.CompareAndSwapInt32(&t.status, waitting, removed) {
		t.t.Load().(*time.Timer).Stop()
		if t.index > 0 {
			this.Lock()
			delete(this.index2Timer, t.index)
			this.Unlock()
		}
		return true
	} else {
		atomic.StoreInt32(&t.status, removed)
		return false
	}
}

func (this *p) removeByIndex(index uint64) (bool, interface{}) {
	this.Lock()
	defer this.Unlock()
	t, ok := this.index2Timer[index]
	if ok {
		if atomic.CompareAndSwapInt32(&t.status, waitting, removed) {
			t.t.Load().(*time.Timer).Stop()
			delete(this.index2Timer, t.index)
			return true, t.ctx
		} else {
			atomic.StoreInt32(&t.status, removed)
			return false, t.ctx
		}
	} else {
		return false, nil
	}
}

//一次性定时器
func (this *p) Once(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.newTimer(timeout, false, eventQue, callback, ctx, 0)
}

func (this *p) OnceWithIndex(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	if index > 0 {
		return this.newTimer(timeout, false, eventQue, callback, ctx, index)
	} else {
		return nil
	}
}

//重复定时器
func (this *p) Repeat(duration time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.newTimer(duration, true, eventQue, callback, ctx, 0)
}

type TimerMgr struct {
	slots []*p
}

func NewTimerMgr() *TimerMgr {

	m := &TimerMgr{
		slots: make([]*p, runtime.NumCPU()*2),
	}

	for i, _ := range m.slots {
		m.slots[i] = newp()
	}

	return m
}

//一次性定时器
func (this *TimerMgr) Once(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.slots[0].newTimer(timeout, false, eventQue, callback, ctx, 0)
}

func (this *TimerMgr) OnceWithIndex(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	if index > 0 {
		slot := int(index) % len(this.slots)
		return this.slots[slot].newTimer(timeout, false, eventQue, callback, ctx, index)
	} else {
		return nil
	}
}

//重复定时器
func (this *TimerMgr) Repeat(duration time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.slots[0].newTimer(duration, true, eventQue, callback, ctx, 0)
}

func (this *TimerMgr) GetTimerByIndex(index uint64) *Timer {
	slot := int(index) % len(this.slots)
	return this.slots[slot].GetTimerByIndex(index)
}

func (this *TimerMgr) CancelByIndex(index uint64) (bool, interface{}) {
	slot := int(index) % len(this.slots)
	return this.slots[slot].removeByIndex(index)
}

/*
 *  终止定时器
 *  注意：因为定时器在单独go程序中调度，Cancel不保证能终止定时器的下次执行（例如定时器马上将要被调度执行，此时在另外
 *        一个go程中调用Cancel），对于重复定时器，可以保证定时器最多在执行一次之后终止。
 * /
func (this *Timer) Cancel() bool {
	return this.p.remove(this)
}

//只对一次性定时器有效
func (this *Timer) ResetFireTime(timeout time.Duration) bool {
	return this.p.resetFireTime(this, timeout)
}

//只对重复定时器有效
func (this *Timer) ResetDuration(duration time.Duration) bool {
	return this.p.resetDuration(this, duration)
}

//一次性定时器
func Once(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return globalMgr.Once(timeout, eventQue, callback, ctx)
}

//重复定时器
func Repeat(duration time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return globalMgr.Repeat(duration, eventQue, callback, ctx)
}

func OnceWithIndex(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	return globalMgr.OnceWithIndex(timeout, eventQue, callback, ctx, index)
}

func GetTimerByIndex(index uint64) *Timer {
	return globalMgr.GetTimerByIndex(index)
}

func CancelByIndex(index uint64) (bool, interface{}) {
	return globalMgr.CancelByIndex(index)
}
*/

package timer

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/util"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

var (
	once      sync.Once
	globalMgr *TimerMgr
)

type TimerMgr struct {
	slots []*p
}

func NewTimerMgr() *TimerMgr {

	m := &TimerMgr{
		slots: make([]*p, runtime.NumCPU()*2),
	}

	for i, _ := range m.slots {
		m.slots[i] = newp()
	}

	return m
}

//一次性定时器
func (this *TimerMgr) Once(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.slots[0].newTimer(timeout, false, eventQue, callback, ctx, 0)
}

func (this *TimerMgr) OnceWithIndex(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	if index > 0 {
		slot := int(index) % len(this.slots)
		return this.slots[slot].newTimer(timeout, false, eventQue, callback, ctx, index)
	} else {
		return nil
	}
}

//重复定时器
func (this *TimerMgr) Repeat(duration time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.slots[0].newTimer(duration, true, eventQue, callback, ctx, 0)
}

func (this *TimerMgr) GetTimerByIndex(index uint64) *Timer {
	slot := int(index) % len(this.slots)
	return this.slots[slot].GetTimerByIndex(index)
}

func (this *TimerMgr) CancelByIndex(index uint64) (bool, interface{}) {
	slot := int(index) % len(this.slots)
	return this.slots[slot].removeByIndex(index)
}

type p struct {
	sync.Mutex
	notiChan    *util.Notifyer
	minheap     util.MinHeap
	index2Timer map[uint64]*Timer
}

func newp() *p {
	mgr := &p{
		notiChan:    util.NewNotifyer(),
		minheap:     util.NewMinHeap(4096),
		index2Timer: map[uint64]*Timer{},
	}
	go mgr.loop()
	return mgr
}

func (this *p) setTimer(t *Timer, index uint64) {
	this.Lock()
	defer this.Unlock()
	if !t.canceled {
		t.expired = time.Now().Add(t.duration)
		if index > 0 {
			this.index2Timer[index] = t
		}
		this.minheap.Insert(t)
		if t == this.minheap.Min().(*Timer) {
			this.notiChan.Notify()
		}
	}
}

func (this *p) GetTimerByIndex(index uint64) *Timer {
	this.Lock()
	defer this.Unlock()
	if t, ok := this.index2Timer[index]; ok {
		return t
	} else {
		return nil
	}
}

func (this *p) resetFireTime(t *Timer, timeout time.Duration) bool {
	if t.repeat {
		return false
	}
	this.Lock()
	defer this.Unlock()
	if t.canceled || t.GetIndex() == -1 {
		return false
	} else {
		t.duration = timeout
		t.expired = time.Now().Add(t.duration)
		this.minheap.Fix(t)
		if t == this.minheap.Min().(*Timer) {
			this.notiChan.Notify()
		}
		return true
	}
}

func (this *p) resetDuration(t *Timer, duration time.Duration) bool {
	if !t.repeat {
		return false
	}
	this.Lock()
	defer this.Unlock()
	if t.canceled {
		return false
	} else {
		t.duration = duration
		t.expired = time.Now().Add(t.duration)
		if t.GetIndex() != -1 {
			this.minheap.Fix(t)
			if t == this.minheap.Min().(*Timer) {
				this.notiChan.Notify()
			}
		}
		return true
	}
}

func (this *p) loop() {
	defaultSleepTime := 10 * time.Second
	var tt *time.Timer
	var min util.HeapElement
	for {
		now := time.Now()
		this.Lock()
		for {
			min = this.minheap.Min()
			if nil != min && now.After(min.(*Timer).expired) {
				t := min.(*Timer)
				this.minheap.PopMin()
				if !t.repeat && t.index > 0 {
					delete(this.index2Timer, t.index)
				}
				this.Unlock()
				t.call()
				this.Lock()
			} else {
				break
			}
		}

		sleepTime := defaultSleepTime
		if nil != min {
			sleepTime = min.(*Timer).expired.Sub(now)
		}
		if nil != tt {
			tt.Reset(sleepTime)
		} else {
			tt = time.AfterFunc(sleepTime, func() {
				this.notiChan.Notify()
			})
		}
		this.Unlock()

		this.notiChan.Wait()
		tt.Stop()
	}
}

func (this *p) clearAll() {
	this.Lock()
	this.Unlock()
	this.minheap.Clear()
}

/*
 *  timeout:    超时时间
 *  repeat:     是否重复定时器
 *  eventQue:   如果非nil,callback会被投递到eventQue，否则在定时器主循环中执行
 */

func (this *p) newTimer(timeout time.Duration, repeat bool, eventQue *event.EventQueue, fn func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	if nil != fn {
		t := &Timer{
			duration: timeout,
			repeat:   repeat,
			callback: fn,
			eventQue: eventQue,
			p:        this,
			ctx:      ctx,
			index:    index,
		}
		this.setTimer(t, index)
		return t
	} else {
		return nil
	}
}

//一次性定时器
func (this *p) Once(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.newTimer(timeout, false, eventQue, callback, ctx, 0)
}

func (this *p) OnceWithIndex(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	if index > 0 {
		return this.newTimer(timeout, false, eventQue, callback, ctx, index)
	} else {
		return nil
	}
}

//重复定时器
func (this *p) Repeat(duration time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	return this.newTimer(duration, true, eventQue, callback, ctx, 0)
}

func (this *p) remove(t *Timer) bool {
	this.Lock()
	defer this.Unlock()

	if t.canceled {
		return false
	}

	t.canceled = true
	if t.GetIndex() == -1 {
		return false
	} else {
		if t.index > 0 {
			delete(this.index2Timer, t.index)
		}
		this.minheap.Remove(t)
		return true
	}
}

func (this *p) removeByIndex(index uint64) (bool, interface{}) {
	this.Lock()
	defer this.Unlock()
	if t, ok := this.index2Timer[index]; ok {
		if t.canceled {
			return false, t.ctx
		}
		t.canceled = true
		if t.GetIndex() == -1 {
			return false, t.ctx
		} else {
			delete(this.index2Timer, t.index)
			this.minheap.Remove(t)
			return true, t.ctx
		}
	} else {
		return false, nil
	}
}

type Timer struct {
	heapIdx  int
	expired  time.Time //到期时间
	eventQue *event.EventQueue
	duration time.Duration
	repeat   bool //是否重复定时器
	canceled bool
	callback func(*Timer, interface{})
	p        *p
	ctx      interface{}
	index    uint64
}

func (this *Timer) Less(o util.HeapElement) bool {
	return o.(*Timer).expired.After(this.expired)
}

func (this *Timer) GetIndex() int {
	return this.heapIdx
}

func (this *Timer) SetIndex(idx int) {
	this.heapIdx = idx
}

func (this *Timer) GetCTX() interface{} {
	return this.ctx
}

func (this *Timer) call_() {

	if _, err := util.ProtectCall(this.callback, this, this.ctx); nil != err {
		logger := kendynet.GetLogger()
		if nil != logger {
			logger.Errorln("error on timer:", err.Error())
		} else {
			fmt.Println("error on timer:", err.Error())
		}
	}

	if this.repeat {
		this.p.setTimer(this, 0)
	}
}

func (this *Timer) call() {
	if nil == this.eventQue {
		this.call_()
	} else {
		this.eventQue.PostNoWait(func() {
			this.call_()
		})
	}
}

/*
 *  终止定时器
 *  注意：因为定时器在单独go程序中调度，Cancel不保证能终止定时器的下次执行（例如定时器马上将要被调度执行，此时在另外
 *        一个go程中调用Cancel），对于重复定时器，可以保证定时器最多在执行一次之后终止。
 */
func (this *Timer) Cancel() bool {
	return this.p.remove(this)
}

//只对一次性定时器有效
func (this *Timer) ResetFireTime(timeout time.Duration) bool {
	return this.p.resetFireTime(this, timeout)
}

//只对重复定时器有效
func (this *Timer) ResetDuration(duration time.Duration) bool {
	return this.p.resetDuration(this, duration)
}

//一次性定时器
func Once(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	once.Do(func() {
		globalMgr = NewTimerMgr()
	})
	return globalMgr.Once(timeout, eventQue, callback, ctx)
}

//重复定时器
func Repeat(duration time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}) *Timer {
	once.Do(func() {
		globalMgr = NewTimerMgr()
	})
	return globalMgr.Repeat(duration, eventQue, callback, ctx)
}

func OnceWithIndex(timeout time.Duration, eventQue *event.EventQueue, callback func(*Timer, interface{}), ctx interface{}, index uint64) *Timer {
	once.Do(func() {
		globalMgr = NewTimerMgr()
	})
	return globalMgr.OnceWithIndex(timeout, eventQue, callback, ctx, index)
}

func GetTimerByIndex(index uint64) *Timer {
	once.Do(func() {
		globalMgr = NewTimerMgr()
	})
	return globalMgr.GetTimerByIndex(index)
}

func CancelByIndex(index uint64) (bool, interface{}) {
	once.Do(func() {
		globalMgr = NewTimerMgr()
	})
	return globalMgr.CancelByIndex(index)
}

package socket

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

var (
	ErrQueueClosed = errors.New("queue closed")
	ErrQueueFull   = errors.New("queue full")
	ErrAddTimeout  = errors.New("add timeout")
)

const (
	initCap         = 64
	defaultFullSize = 10000
)

type stWait struct {
	listEle *list.Element
	ch      chan struct{}
}

//实现cond以支持带timeout的wait
type Cond struct {
	sync.Mutex
	mu   *sync.Mutex
	wait *list.List
}

func NewCond(mu *sync.Mutex) *Cond {
	return &Cond{
		mu:   mu,
		wait: list.New(),
	}
}

func (c *Cond) Wait() {
	w := &stWait{
		ch: make(chan struct{}),
	}
	c.Lock()
	w.listEle = c.wait.PushBack(w)
	c.Unlock()
	c.mu.Unlock()
	<-w.ch
	c.mu.Lock()
}

func (c *Cond) WaitWithTimeout(timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}

	w := &stWait{
		ch: make(chan struct{}),
	}

	c.Lock()
	w.listEle = c.wait.PushBack(w)
	c.Unlock()

	c.mu.Unlock()

	ticker := time.NewTicker(timeout)

	ok := false

	select {
	case <-w.ch:
		ticker.Stop()
		ok = true
	case <-ticker.C:
		c.Lock()
		c.wait.Remove(w.listEle)
		c.Unlock()
	}
	c.mu.Lock()
	return ok
}

func (c *Cond) Signal() {
	c.Lock()
	if c.wait.Len() > 0 {
		v := c.wait.Remove(c.wait.Front()).(*stWait)
		close(v.ch)
	}
	c.Unlock()
}

func (c *Cond) Broadcast() {
	c.Lock()
	for n := c.wait.Front(); nil != n; n = c.wait.Front() {
		v := c.wait.Remove(n)
		close(v.(*stWait).ch)

	}
	c.Unlock()
}

type SendQueue struct {
	list        []interface{}
	listGuard   sync.Mutex
	emptyCond   *Cond
	fullCond    *Cond
	fullSize    int
	closed      bool
	emptyWaited int
	fullWaited  int
}

//如果队列满返回busy
func (self *SendQueue) Add(item interface{}) error {
	self.listGuard.Lock()
	if self.closed {
		self.listGuard.Unlock()
		return ErrQueueClosed
	}

	if len(self.list) >= self.fullSize {
		self.listGuard.Unlock()
		return ErrQueueFull
	}

	self.list = append(self.list, item)

	needSignal := self.emptyWaited > 0
	self.listGuard.Unlock()
	if needSignal {
		self.emptyCond.Signal()
	}
	return nil
}

func (self *SendQueue) AddWithTimeout(item interface{}, timeout time.Duration) error {
	self.listGuard.Lock()
	if self.closed {
		self.listGuard.Unlock()
		return ErrQueueClosed
	}

	for len(self.list) >= self.fullSize {
		if 0 == timeout {
			self.fullWaited++
			self.fullCond.Wait()
			self.fullWaited--
			if self.closed {
				self.listGuard.Unlock()
				return ErrQueueClosed
			}
		} else {
			self.fullWaited++
			ok := self.fullCond.WaitWithTimeout(timeout)
			self.fullWaited--
			if self.closed {
				self.listGuard.Unlock()
				return ErrQueueClosed
			} else if !ok {
				self.listGuard.Unlock()
				return ErrAddTimeout
			}
		}
	}

	self.list = append(self.list, item)

	needSignal := self.emptyWaited > 0
	self.listGuard.Unlock()
	if needSignal {
		self.emptyCond.Signal()
	}
	return nil
}

func (self *SendQueue) Get(swaped []interface{}) (closed bool, datas []interface{}) {
	swaped = swaped[0:0]
	self.listGuard.Lock()
	for !self.closed && len(self.list) == 0 {
		self.emptyWaited++
		self.emptyCond.Wait()
		self.emptyWaited--
	}
	datas = self.list
	closed = self.closed
	needSignal := self.fullWaited > 0
	self.list = swaped
	self.listGuard.Unlock()
	if needSignal {
		self.fullCond.Broadcast()
	}
	return
}

func (self *SendQueue) Close() (bool, int) {
	self.listGuard.Lock()
	n := len(self.list)
	if self.closed {
		self.listGuard.Unlock()
		return false, n
	}

	self.closed = true
	self.listGuard.Unlock()
	self.emptyCond.Broadcast()
	self.fullCond.Broadcast()

	return true, n
}

func (self *SendQueue) SetFullSize(newSize int) {
	if newSize > 0 {
		needSignal := false
		self.listGuard.Lock()
		oldSize := self.fullSize
		self.fullSize = newSize
		if oldSize < newSize && self.fullWaited > 0 {
			needSignal = true
		}
		self.listGuard.Unlock()
		if needSignal {
			self.fullCond.Broadcast()
		}
	}
}

func (self *SendQueue) Closed() bool {
	var closed bool
	self.listGuard.Lock()
	closed = self.closed
	self.listGuard.Unlock()
	return closed
}

func (self *SendQueue) Empty() bool {
	self.listGuard.Lock()
	defer self.listGuard.Unlock()
	return len(self.list) == 0
}

func NewSendQueue(fullSize ...int) *SendQueue {
	self := &SendQueue{}
	self.closed = false
	self.emptyCond = NewCond(&self.listGuard)
	self.fullCond = NewCond(&self.listGuard)
	self.list = make([]interface{}, 0, initCap)

	if len(fullSize) > 0 {
		if fullSize[0] <= 0 {
			return nil
		}
		self.fullSize = fullSize[0]
	} else {
		self.fullSize = defaultFullSize
	}

	return self
}

package socket

import (
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
	nnext *stWait
	pprev *stWait
	ch    chan struct{}
}

var stWaitPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		return &stWait{}
	},
}

func getStWait() *stWait {
	return stWaitPool.Get().(*stWait)
}

func putStWait(i *stWait) {
	i.ch = nil
	stWaitPool.Put(i)
}

func pushWait(head *stWait, ch chan struct{}) *stWait {
	n := getStWait()
	n.ch = ch

	tail := head.pprev

	n.nnext = tail.nnext
	n.pprev = tail

	tail.nnext = n
	head.pprev = n

	return n
}

func popWait(head *stWait) chan struct{} {
	if head.nnext == head {
		return nil
	} else {
		first := head.nnext
		ch := first.ch
		removeWait(first)
		return ch
	}
}

func removeWait(n *stWait) {
	if nil != n.nnext && nil != n.pprev && n.nnext != n && n.pprev != n {
		next := n.nnext
		prev := n.pprev
		prev.nnext = next
		next.pprev = prev
		n.nnext = nil
		n.pprev = nil
		putStWait(n)
	}
}

//实现cond以支持带timeout的wait
type cond struct {
	sync.Mutex
	mu       *sync.Mutex
	waitList *stWait
}

func newCond(mu *sync.Mutex) *cond {
	c := &cond{
		mu:       mu,
		waitList: &stWait{},
	}

	c.waitList.nnext = c.waitList
	c.waitList.pprev = c.waitList
	return c
}

func (c *cond) wait() {

	ch := make(chan struct{})
	c.Lock()
	pushWait(c.waitList, ch)
	c.Unlock()
	c.mu.Unlock()
	<-ch
	c.mu.Lock()
}

func (c *cond) waitWithTimeout(timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}

	ch := make(chan struct{})

	c.Lock()
	w := pushWait(c.waitList, ch)
	c.Unlock()

	c.mu.Unlock()

	ticker := time.NewTicker(timeout)

	ok := false

	select {
	case <-ch:
		ticker.Stop()
		ok = true
	case <-ticker.C:
		c.Lock()
		removeWait(w)
		c.Unlock()
	}
	c.mu.Lock()
	return ok
}

func (c *cond) signal() {
	c.Lock()
	ch := popWait(c.waitList)
	c.Unlock()
	if nil != ch {
		close(ch)
	}
}

func (c *cond) broadcast() {
	c.Lock()
	waitList := c.waitList
	c.waitList = &stWait{}
	c.waitList.nnext = c.waitList
	c.waitList.pprev = c.waitList
	c.Unlock()
	for ch := popWait(waitList); nil != ch; ch = popWait(waitList) {
		close(ch)
	}
}

type SendQueue struct {
	list        []interface{}
	listGuard   sync.Mutex
	emptyCond   *cond
	fullCond    *cond
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
		self.emptyCond.signal()
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
		if timeout > 0 {
			self.fullWaited++
			ok := self.fullCond.waitWithTimeout(timeout)
			self.fullWaited--
			if self.closed {
				self.listGuard.Unlock()
				return ErrQueueClosed
			} else if !ok {
				self.listGuard.Unlock()
				return ErrAddTimeout
			}
		} else {
			self.fullWaited++
			self.fullCond.wait()
			self.fullWaited--
			if self.closed {
				self.listGuard.Unlock()
				return ErrQueueClosed
			}
		}
	}

	self.list = append(self.list, item)

	needSignal := self.emptyWaited > 0
	self.listGuard.Unlock()
	if needSignal {
		self.emptyCond.signal()
	}
	return nil
}

func (self *SendQueue) Get(swaped []interface{}) (closed bool, datas []interface{}) {
	swaped = swaped[0:0]
	self.listGuard.Lock()
	for !self.closed && len(self.list) == 0 {
		self.emptyWaited++
		self.emptyCond.wait()
		self.emptyWaited--
	}
	datas = self.list
	closed = self.closed
	needSignal := self.fullWaited > 0
	self.list = swaped
	self.listGuard.Unlock()
	if needSignal {
		self.fullCond.broadcast()
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
	self.emptyCond.broadcast()
	self.fullCond.broadcast()

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
			self.fullCond.broadcast()
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
	self.emptyCond = newCond(&self.listGuard)
	self.fullCond = newCond(&self.listGuard)
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

package kendynet

import (
	"sync"
	"fmt"
)

var (
	ErrSendQueueClosed = fmt.Errorf("send queue closed")
)

type SendQueue struct {
	list      [] Message
	listGuard sync.Mutex
	listCond  *sync.Cond
	closed     bool
	waited     int
}

func (self *SendQueue) Add(item Message) error {
	self.listGuard.Lock()
	if self.closed {
		self.listGuard.Unlock()
		return ErrSendQueueClosed
	}
	n := len(self.list)
	self.list = append(self.list, item)
	needSignal := self.waited > 0 && n == 0/*BlockQueue目前主要用于单消费者队列，这里n == 0的处理是为了这种情况的优化,减少Signal的调用次数*/
	self.listGuard.Unlock()
	if needSignal {
		self.listCond.Signal()
	}
	return nil
}

func (self *SendQueue) Closed() bool {
	var closed bool
	self.listGuard.Lock()
	closed = self.closed
	self.listGuard.Unlock()
	return closed
}

func (self *SendQueue) Get() (closed bool,datas []Message) {
	self.listGuard.Lock()
	for !self.closed && len(self.list) == 0 {
		//Cond.Wait不能设置超时，蛋疼
		self.waited++
		self.listCond.Wait()
		self.waited--
	}
	if len(self.list) > 0 {
		datas  = self.list
		self.list = make([]Message,0)
	}

	closed = self.closed
	self.listGuard.Unlock()
	return
}

func (self *SendQueue) Swap(swaped []Message) (closed bool,datas []Message) {
	swaped = swaped[0:0]
	self.listGuard.Lock()
	for !self.closed && len(self.list) == 0 {
		self.waited++
		//Cond.Wait不能设置超时，蛋疼
		self.listCond.Wait()
		self.waited--
	}
	datas  = self.list
	closed = self.closed
	self.list = swaped
	self.listGuard.Unlock()
	return
}

func (self *SendQueue) Close() {
	self.listGuard.Lock()

	if self.closed {
		self.listGuard.Unlock()
		return
	}

	self.closed = true
	self.listGuard.Unlock()
	self.listCond.Signal()
}

func (self *SendQueue) Len() (length int) {
	self.listGuard.Lock()
	length = len(self.list)
	self.listGuard.Unlock()
	return
}

func (self *SendQueue) Clear() {
	self.listGuard.Lock()
	self.list = self.list[0:0]
	self.listGuard.Unlock()
	return
}

func NewSendQueue() *SendQueue {
	self := &SendQueue{}
	self.closed = false
	self.listCond = sync.NewCond(&self.listGuard)

	return self
}
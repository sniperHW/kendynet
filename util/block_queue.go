package util

import (
	"sync"
	"fmt"
)

var (
	ErrQueueClosed = fmt.Errorf("queue closed")
)

type BlockQueue struct {
	list      [] interface{}
	listGuard sync.Mutex
	listCond  *sync.Cond
	closed     bool
}

func (self *BlockQueue) Add(item interface{}) error {
	self.listGuard.Lock()

	if self.closed {
		self.listGuard.Unlock()
		return ErrQueueClosed
	}

	self.list = append(self.list, item)
	self.listGuard.Unlock()
	self.listCond.Signal()

	return nil
}

func (self *BlockQueue) Closed() bool {
	var closed bool
	self.listGuard.Lock()
	closed = self.closed
	self.listGuard.Unlock()
	return closed
}

func (self *BlockQueue) Get(out *List) bool {
	self.listGuard.Lock()
	for !self.closed && len(self.list) == 0 {
		//Cond.Wait不能设置超时，蛋疼
		self.listCond.Wait()
	}

	for _, p := range self.list {
		out.Push(p)
	}

	self.list = self.list[0:0]

	self.listGuard.Unlock()

	return self.closed

}

func (self *BlockQueue) Close() {
	self.listGuard.Lock()

	if self.closed {
		self.listGuard.Unlock()
		return
	}

	self.closed = true
	self.listGuard.Unlock()
	self.listCond.Signal()
}

func (self *BlockQueue) Len() (length int) {
	self.listGuard.Lock()
	length = len(self.list)
	self.listGuard.Unlock()
	return
}

func (self *BlockQueue) Clear() {
	self.listGuard.Lock()
	self.list = self.list[0:0]
	self.listGuard.Unlock()
	return
}

func NewBlockQueue() *BlockQueue {
	self := &BlockQueue{}
	self.closed = false
	self.listCond = sync.NewCond(&self.listGuard)

	return self
}
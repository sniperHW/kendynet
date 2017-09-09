package util

import (
	"sync"
	"fmt"
)

var (
	ErrQueueClosed = fmt.Errorf("queue closed")
)

type Queue struct {
	list      [] interface{}
	listGuard sync.Mutex
	listCond  *sync.Cond
	closed     bool
}

func (self *Queue) Add(item interface{}) error {
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

func (self *Queue) Get(out * [] interface{}) bool {
	self.listGuard.Lock()
	for !self.closed && len(self.list) == 0 {
		//Cond.Wait不能设置超时，蛋疼
		self.listCond.Wait()
	}

	for _, p := range self.list {
		*out = append(*out, p)
	}

	self.list = self.list[0:0]

	self.listGuard.Unlock()

	return self.closed

}

func (self *Queue) Close() {
	self.listGuard.Lock()

	if self.closed {
		self.listGuard.Unlock()
		return
	}

	self.closed = true
	self.listGuard.Unlock()
	self.listCond.Signal()
}

func NewQueue() *Queue {
	self := &Queue{}
	self.closed = false
	self.listCond = sync.NewCond(&self.listGuard)

	return self
}
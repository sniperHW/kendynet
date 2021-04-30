package aio

import (
	"sync"
)

var itemPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		return &item{}
	},
}

type item struct {
	nnext *item
	v     interface{}
}

type sendqueue struct {
	head *item
	tail *item
	cap  int
	len  int
}

func newSendQueue(cap int) sendqueue {
	return sendqueue{
		cap: cap,
	}
}

func (r *sendqueue) empty() bool {
	return r.len == 0
}

func (r *sendqueue) setCap(cap int) {
	r.cap = cap
}

func (r *sendqueue) pop() interface{} {
	if r.len == 0 {
		return nil
	} else {
		f := r.head
		v := f.v
		r.head = f.nnext
		if r.head == nil {
			r.tail = nil
		}
		f.nnext = nil
		f.v = nil
		itemPool.Put(f)
		r.len--
		return v
	}
}

func (r *sendqueue) push(v interface{}) bool {
	if r.len == r.cap {
		return false
	} else {
		it := itemPool.Get().(*item)
		it.v = v
		if nil == r.tail {
			r.head = it
		} else {
			r.tail.nnext = it
		}
		r.tail = it
		r.len++
		return true
	}
}

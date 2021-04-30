package util

import (
	"sync/atomic"
)

type Flag uint32

func (this *Flag) AtomicSet(flag uint32) {
	for {
		f := atomic.LoadUint32((*uint32)(this))
		if atomic.CompareAndSwapUint32((*uint32)(this), f, f|flag) {
			break
		}
	}
}

func (this *Flag) AtomicClear(flag uint32) {
	for {
		f := atomic.LoadUint32((*uint32)(this))
		if atomic.CompareAndSwapUint32((*uint32)(this), f, f&(flag^0xFFFFFFFF)) {
			break
		}
	}
}

func (this *Flag) AtomicTest(flag uint32) bool {
	return atomic.LoadUint32((*uint32)(this))&flag > 0
}

func (this *Flag) Test(flag uint32) bool {
	return (uint32)(*this)&flag > 0
}

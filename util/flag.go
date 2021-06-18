package util

import (
	"sync/atomic"
)

type Flag uint32

func (this *Flag) AtomicSet(flag uint32) (ret uint32) {
	for {
		f := atomic.LoadUint32((*uint32)(this))
		if atomic.CompareAndSwapUint32((*uint32)(this), f, f|flag) {
			ret = f | flag
			break
		}
	}
	return
}

func (this *Flag) AtomicClear(flag uint32) (ret uint32) {
	for {
		f := atomic.LoadUint32((*uint32)(this))
		if atomic.CompareAndSwapUint32((*uint32)(this), f, f&(flag^0xFFFFFFFF)) {
			ret = f & (flag ^ 0xFFFFFFFF)
			break
		}
	}
	return
}

func (this *Flag) AtomicTest(flag uint32) bool {
	return atomic.LoadUint32((*uint32)(this))&flag > 0
}

func (this *Flag) Test(flag uint32) bool {
	return (uint32)(*this)&flag > 0
}

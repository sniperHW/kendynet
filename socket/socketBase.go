package socket

import (
	"errors"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"net"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	fclosed  = uint32(1 << 1) //是否已经调用Close
	frclosed = uint32(1 << 2) //调用来了shutdownRead
	fdoclose = uint32(1 << 3)
	fdoingW  = uint32(1 << 4)
	fdoingR  = uint32(1 << 5)
)

type SocketImpl interface {
	kendynet.StreamSession
	recvThreadFunc()
	sendThreadFunc()
	defaultInBoundProcessor() kendynet.InBoundProcessor
	getInBoundProcessor() kendynet.InBoundProcessor
	SetInBoundProcessor(kendynet.InBoundProcessor) kendynet.StreamSession
	getNetConn() net.Conn
}

type SocketBase struct {
	flag            util.Flag
	ud              atomic.Value
	ioCount         int32
	sendQue         *SendQueue
	sendTimeout     int64
	recvTimeout     int64
	sendCloseChan   chan struct{}
	imp             SocketImpl
	closeOnce       int32
	beginOnce       int32
	sendOnce        int32
	doCloseOnce     int32
	closeReason     error
	encoder         kendynet.EnCoder
	errorCallback   func(kendynet.StreamSession, error)
	closeCallBack   func(kendynet.StreamSession, error)
	inboundCallBack func(kendynet.StreamSession, interface{})
}

func (this *SocketBase) IsClosed() bool {
	return this.flag.AtomicTest(fclosed)
}

func (this *SocketBase) LocalAddr() net.Addr {
	return this.imp.getNetConn().LocalAddr()
}

func (this *SocketBase) RemoteAddr() net.Addr {
	return this.imp.getNetConn().RemoteAddr()
}

func (this *SocketBase) SetUserData(ud interface{}) kendynet.StreamSession {
	this.ud.Store(ud)
	return this.imp
}

func (this *SocketBase) GetUserData() interface{} {
	return this.ud.Load()
}

func (this *SocketBase) SetRecvTimeout(timeout time.Duration) kendynet.StreamSession {
	atomic.StoreInt64(&this.recvTimeout, int64(timeout))
	return this.imp
}

func (this *SocketBase) SetSendTimeout(timeout time.Duration) kendynet.StreamSession {
	atomic.StoreInt64(&this.sendTimeout, int64(timeout))
	return this.imp
}

func (this *SocketBase) SetErrorCallBack(cb func(kendynet.StreamSession, error)) kendynet.StreamSession {
	this.errorCallback = cb
	return this.imp
}

func (this *SocketBase) SetCloseCallBack(cb func(kendynet.StreamSession, error)) kendynet.StreamSession {
	this.closeCallBack = cb
	return this.imp
}

func (this *SocketBase) SetEncoder(encoder kendynet.EnCoder) kendynet.StreamSession {
	this.encoder = encoder
	return this.imp
}

func (this *SocketBase) SetSendQueueSize(size int) kendynet.StreamSession {
	this.sendQue.SetFullSize(size)
	return this.imp
}

func (this *SocketBase) getRecvTimeout() time.Duration {
	return time.Duration(atomic.LoadInt64(&this.recvTimeout))
}

func (this *SocketBase) getSendTimeout() time.Duration {
	return time.Duration(atomic.LoadInt64(&this.sendTimeout))
}

func (this *SocketBase) Send(o interface{}) error {
	if nil == o {
		return kendynet.ErrInvaildObject
	} else if _, ok := o.([]byte); !ok && nil == this.encoder {
		return kendynet.ErrInvaildEncoder
	} else {

		if err := this.sendQue.Add(o); nil != err {
			if err == ErrQueueClosed {
				err = kendynet.ErrSocketClose
			} else if err == ErrQueueFull {
				err = kendynet.ErrSendQueFull
			}
			return err
		}

		if atomic.CompareAndSwapInt32(&this.sendOnce, 0, 1) {
			this.addIO()
			go this.imp.sendThreadFunc()
		}
		return nil
	}
}

func (this *SocketBase) SendWithTimeout(o interface{}, timeout time.Duration) error {
	if nil == o {
		return kendynet.ErrInvaildObject
	} else if _, ok := o.([]byte); !ok && nil == this.encoder {
		return kendynet.ErrInvaildEncoder
	} else {

		if err := this.sendQue.AddWithTimeout(o, timeout); nil != err {
			if err == ErrQueueClosed {
				err = kendynet.ErrSocketClose
			} else if err == ErrQueueFull {
				err = kendynet.ErrSendQueFull
			} else if err == ErrAddTimeout {
				err = kendynet.ErrSendTimeout
			}
			return err
		}

		if atomic.CompareAndSwapInt32(&this.sendOnce, 0, 1) {
			this.addIO()
			go this.imp.sendThreadFunc()
		}
		return nil
	}
}

func (this *SocketBase) BeginRecv(cb func(kendynet.StreamSession, interface{})) (err error) {

	if atomic.CompareAndSwapInt32(&this.beginOnce, 0, 1) {

		if nil == cb {
			err = errors.New("cb is nil")
		} else {
			if this.flag.AtomicTest(fclosed | frclosed) {
				err = kendynet.ErrSocketClose
			} else {
				if nil == this.imp.getInBoundProcessor() {
					this.imp.SetInBoundProcessor(this.imp.defaultInBoundProcessor())
				}
				this.addIO()
				this.inboundCallBack = cb
				go this.imp.recvThreadFunc()
			}
		}
	}

	return
}

func (this *SocketBase) addIO() {
	atomic.AddInt32(&this.ioCount, 1)
}

func (this *SocketBase) ioDone() {
	if atomic.AddInt32(&this.ioCount, -1) == 0 && this.flag.AtomicTest(fdoclose) {
		if atomic.CompareAndSwapInt32(&this.doCloseOnce, 0, 1) {
			if nil != this.closeCallBack {
				this.closeCallBack(this.imp, this.closeReason)
			}
		}
	}
}

func (this *SocketBase) ShutdownRead() {
	this.flag.AtomicSet(frclosed)
	this.imp.getNetConn().(interface{ CloseRead() error }).CloseRead()
}

func (this *SocketBase) ShutdownWrite() {
	closeOK, remain := this.sendQue.Close()
	if closeOK && remain == 0 {
		this.imp.getNetConn().(interface{ CloseWrite() error }).CloseWrite()
	}
}

func (this *SocketBase) Close(reason error, delay time.Duration) {

	if atomic.CompareAndSwapInt32(&this.closeOnce, 0, 1) {
		runtime.SetFinalizer(this.imp, nil)

		this.flag.AtomicSet(fclosed)

		_, remain := this.sendQue.Close()

		if remain > 0 && delay > 0 {
			this.ShutdownRead()
			ticker := time.NewTicker(delay)
			go func() {
				/*
				 *	delay > 0,sendThread最多需要经过delay秒之后才会结束，
				 *	为了避免阻塞调用Close的goroutine,启动一个新的goroutine在chan上等待事件
				 */
				select {
				case <-this.sendCloseChan:
				case <-ticker.C:
				}
				ticker.Stop()
				this.imp.getNetConn().Close()
			}()

		} else {
			this.imp.getNetConn().Close()
		}

		this.closeReason = reason
		this.flag.AtomicSet(fdoclose)

		if atomic.LoadInt32(&this.ioCount) == 0 {
			if atomic.CompareAndSwapInt32(&this.doCloseOnce, 0, 1) {
				if nil != this.closeCallBack {
					this.closeCallBack(this.imp, reason)
				}
			}
		}
	}
}

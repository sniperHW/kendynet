package socket

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"io"
	"net"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	started = int32(1 << 0)
	closed  = int32(1 << 1)
	wclosed = int32(1 << 2)
	rclosed = int32(1 << 3)
)

type SocketImpl interface {
	kendynet.StreamSession
	recvThreadFunc()
	sendThreadFunc()
	getNetConn() net.Conn
	sendMessage(kendynet.Message) error
	defaultReceiver() kendynet.Receiver
}

type SocketBase struct {
	ud            atomic.Value
	sendQue       *util.BlockQueue
	receiver      kendynet.Receiver
	encoder       *kendynet.EnCoder
	flag          int32
	sendTimeout   atomic.Value
	recvTimeout   atomic.Value
	onClose       atomic.Value
	onEvent       func(*kendynet.Event)
	closeReason   string
	sendCloseChan chan struct{}
	imp           SocketImpl
}

func (this *SocketBase) setFlag(flag int32) {
	for !atomic.CompareAndSwapInt32(&this.flag, this.flag, this.flag|flag) {
	}
}

func (this *SocketBase) testFlag(flag int32) bool {
	return atomic.LoadInt32(&this.flag)&flag > 0
}

func (this *SocketBase) IsClosed() bool {
	return this.testFlag(closed)
}

func (this *SocketBase) LocalAddr() net.Addr {
	return this.imp.getNetConn().LocalAddr()
}

func (this *SocketBase) RemoteAddr() net.Addr {
	return this.imp.getNetConn().RemoteAddr()
}

func (this *SocketBase) SetUserData(ud interface{}) {
	this.ud.Store(ud)
}

func (this *SocketBase) GetUserData() interface{} {
	return this.ud.Load()
}

func (this *SocketBase) doClose() {
	this.imp.getNetConn().Close()
	if onClose := this.onClose.Load(); nil != onClose {
		onClose.(func(kendynet.StreamSession, string))(this.imp.(kendynet.StreamSession), this.closeReason)
	}
}

func (this *SocketBase) shutdownRead() {
	underConn := this.imp.getNetConn()
	switch underConn.(type) {
	case *net.TCPConn:
		underConn.(*net.TCPConn).CloseRead()
		break
	case *net.UnixConn:
		underConn.(*net.UnixConn).CloseRead()
		break
	}
}

func (this *SocketBase) ShutdownRead() {
	if !this.testFlag(closed) {
		this.setFlag(rclosed)
		this.shutdownRead()
	}
}

func (this *SocketBase) Start(eventCB func(*kendynet.Event)) error {

	for {

		flag := atomic.LoadInt32(&this.flag)

		if flag&started > 0 {
			return kendynet.ErrStarted
		} else if flag&closed > 0 {
			return kendynet.ErrSocketClose
		}

		if atomic.CompareAndSwapInt32(&this.flag, flag, flag|started) {
			break
		} else {
			fmt.Println(flag, this.flag, this.flag|started)
		}
	}

	if this.receiver == nil {
		this.receiver = this.imp.defaultReceiver()
	}

	this.onEvent = eventCB

	go this.imp.sendThreadFunc()
	go this.imp.recvThreadFunc()

	return nil
}

func (this *SocketBase) SetRecvTimeout(timeout time.Duration) {
	this.recvTimeout.Store(timeout)
}

func (this *SocketBase) SetSendTimeout(timeout time.Duration) {
	this.sendTimeout.Store(timeout)
}

func (this *SocketBase) SetCloseCallBack(cb func(kendynet.StreamSession, string)) {
	this.onClose.Store(cb)
}

func (this *SocketBase) SetEncoder(encoder kendynet.EnCoder) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&this.encoder)), unsafe.Pointer(&encoder))
}

func (this *SocketBase) SetReceiver(r kendynet.Receiver) {
	if !this.testFlag(started) {
		this.receiver = r
	}
}

func (this *SocketBase) SetSendQueueSize(size int) {
	this.sendQue.SetFullSize(size)
}

func (this *SocketBase) Send(o interface{}) error {
	if o == nil {
		return kendynet.ErrInvaildObject
	}

	encoder := (*kendynet.EnCoder)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&this.encoder))))

	if nil == *encoder {
		return kendynet.ErrInvaildEncoder
	}

	msg, err := (*encoder).EnCode(o)

	if err != nil {
		return err
	}

	return this.imp.sendMessage(msg)
}

func (this *SocketBase) SendMessage(msg kendynet.Message) error {
	return this.imp.sendMessage(msg)
}

func (this *SocketBase) recvThreadFunc() {

	conn := this.imp.getNetConn()

	for !this.IsClosed() {

		var (
			p     interface{}
			err   error
			event kendynet.Event
		)

		recvTimeout := this.getRecvTimeout()

		if recvTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(recvTimeout))
			p, err = this.receiver.ReceiveAndUnpack(this.imp)
			conn.SetReadDeadline(time.Time{})
		} else {
			p, err = this.receiver.ReceiveAndUnpack(this.imp)
		}

		if err != nil || p != nil {
			event.Session = this.imp
			if err != nil {
				event.EventType = kendynet.EventTypeError
				event.Data = err
				if err == io.EOF {
					this.setFlag(rclosed)
				} else if kendynet.IsNetTimeout(err) {
					event.Data = kendynet.ErrRecvTimeout
				} else {
					kendynet.GetLogger().Errorf("ReceiveAndUnpack error:%s\n", err.Error())
					this.setFlag(rclosed | wclosed)
				}
			} else {
				event.EventType = kendynet.EventTypeMessage
				event.Data = p
			}

			if !this.IsClosed() {
				this.onEvent(&event)
			}

		}
	}
}

func (this *SocketBase) getRecvTimeout() time.Duration {
	t := this.recvTimeout.Load()
	if nil == t {
		return 0
	} else {
		return t.(time.Duration)
	}
}

func (this *SocketBase) getSendTimeout() time.Duration {
	t := this.sendTimeout.Load()
	if nil == t {
		return 0
	} else {
		return t.(time.Duration)
	}
}

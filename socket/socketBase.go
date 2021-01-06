package socket

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	fstarted    = int32(1 << 0) //是否已经Start
	fclosed     = int32(1 << 1) //是否已经调用Close
	frecvStoped = int32(1 << 4) //recvThread已经结束
	fsendStoped = int32(1 << 5) //sendThread已经结束
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
	flag          int32
	ud            atomic.Value
	sendQue       *util.BlockQueue
	receiver      atomic.Value
	encoder       atomic.Value
	sendTimeout   atomic.Value
	recvTimeout   atomic.Value
	onClose       atomic.Value
	onEvent       func(*kendynet.Event)
	closeReason   string
	sendCloseChan chan struct{}
	imp           SocketImpl
	CBLock        sync.Mutex
}

func (this *SocketBase) setFlag(flag int32) {
	for !atomic.CompareAndSwapInt32(&this.flag, this.flag, this.flag|flag) {
	}
}

func (this *SocketBase) testFlag(flag int32) bool {
	return atomic.LoadInt32(&this.flag)&flag > 0
}

func (this *SocketBase) IsClosed() bool {
	return this.testFlag(fclosed)
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
	if !this.testFlag(fclosed) {
		this.shutdownRead()
	}
}

//保证onEvent在读写线程中按序执行
func (this *SocketBase) callEventCB(event *kendynet.Event) int32 {
	/*
	 *  这个锁在绝大多数情况下无竞争，只有在sendThreadFunc发生错误需要调用onEvent时才可能发生竞争
	 */
	this.CBLock.Lock()
	defer this.CBLock.Unlock()

	if this.testFlag(fclosed) {
		return fclosed
	}

	this.onEvent(event)

	if this.testFlag(fclosed) {
		return fclosed
	} else {
		return 0
	}
}

func (this *SocketBase) Start(eventCB func(*kendynet.Event)) error {

	for {

		flag := atomic.LoadInt32(&this.flag)

		if flag&fclosed > 0 {
			return kendynet.ErrSocketClose
		} else if flag&fstarted > 0 {
			return kendynet.ErrStarted
		}

		if atomic.CompareAndSwapInt32(&this.flag, flag, flag|fstarted) {
			break
		}
	}

	if this.receiver.Load() == nil {
		this.receiver.Store(this.imp.defaultReceiver())
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
	this.encoder.Store(encoder)
}

func (this *SocketBase) SetReceiver(r kendynet.Receiver) {
	if nil != r {
		this.receiver.Store(r)
	}
}

func (this *SocketBase) SetSendQueueSize(size int) {
	this.sendQue.SetFullSize(size)
}

func (this *SocketBase) Send(o interface{}) error {
	if o == nil {
		return kendynet.ErrInvaildObject
	}

	var encoder kendynet.EnCoder
	if v := this.encoder.Load(); nil != v {
		encoder = v.(kendynet.EnCoder)
	} else {
		return kendynet.ErrInvaildEncoder
	}

	msg, err := encoder.EnCode(o)

	if err != nil {
		return err
	}

	return this.imp.sendMessage(msg)
}

func (this *SocketBase) SendMessage(msg kendynet.Message) error {
	return this.imp.sendMessage(msg)
}

func (this *SocketBase) recvThreadFunc() {

	defer func() {
		this.setFlag(frecvStoped)
		if this.testFlag(fsendStoped) {
			if onClose := this.onClose.Load(); nil != onClose {
				onClose.(func(kendynet.StreamSession, string))(this.imp.(kendynet.StreamSession), this.closeReason)
			}
		}
	}()

	conn := this.imp.getNetConn()

	receiver := this.receiver.Load().(kendynet.Receiver)

	isClosed := this.IsClosed()

	for !isClosed {

		var (
			p     interface{}
			err   error
			event kendynet.Event
		)

		recvTimeout := this.getRecvTimeout()

		if recvTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(recvTimeout))
			p, err = receiver.ReceiveAndUnpack(this.imp)
			conn.SetReadDeadline(time.Time{})
		} else {
			p, err = receiver.ReceiveAndUnpack(this.imp)
		}

		if err != nil || p != nil {
			event.Session = this.imp
			if err != nil {
				event.EventType = kendynet.EventTypeError
				event.Data = err
				if kendynet.IsNetTimeout(err) {
					event.Data = kendynet.ErrRecvTimeout
				} else {
					this.sendQue.CloseAndClear()
				}
			} else {
				event.EventType = kendynet.EventTypeMessage
				event.Data = p
			}
			isClosed = (this.callEventCB(&event) == fclosed)
		} else {
			isClosed = this.IsClosed()
		}
	}
}

func (this *SocketBase) Close(reason string, delay time.Duration) {
	var flag int32
	for {
		flag = atomic.LoadInt32(&this.flag)

		if flag&fclosed > 0 {
			return
		}

		if atomic.CompareAndSwapInt32(&this.flag, flag, flag|fclosed) {
			break
		}
	}

	wclosed := this.sendQue.Closed()

	this.sendQue.Close()

	this.closeReason = reason

	if flag&fstarted == 0 {
		this.imp.getNetConn().Close()
		if onClose := this.onClose.Load(); nil != onClose {
			onClose.(func(kendynet.StreamSession, string))(this.imp.(kendynet.StreamSession), this.closeReason)
		}
	} else {

		if wclosed || this.sendQue.Len() == 0 {
			delay = 0 //写端已经关闭，delay参数没有意义设置为0
		} else if delay > 0 {
			delay = delay * time.Second
		}

		if delay > 0 {
			this.shutdownRead()
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
			this.sendQue.Clear()
			this.imp.getNetConn().Close()
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

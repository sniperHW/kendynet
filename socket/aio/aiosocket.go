package aio

import (
	"errors"
	"github.com/sniperHW/goaio"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/sniperHW/kendynet/gopool"
	"github.com/sniperHW/kendynet/socket"
	"github.com/sniperHW/kendynet/util"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type SocketService struct {
	services    []*goaio.AIOService
	shareBuffer goaio.ShareBuffer
}

var sendRoutinePool *gopool.Pool = gopool.New(gopool.Option{
	MaxRoutineCount: 1024,
	Mode:            gopool.QueueMode,
})

var ioResutRoutinePool *gopool.Pool = gopool.New(gopool.Option{
	MaxRoutineCount: 1024,
	Mode:            gopool.GoMode,
})

type sendContext struct {
	b  *buffer.Buffer
	cb func(*goaio.AIOResult, *sendContext)
}

type recvContext struct {
	cb func(*goaio.AIOResult)
}

func (this *SocketService) completeRoutine(s *goaio.AIOService) {
	for {
		res, ok := s.GetCompleteStatus()
		if !ok {
			break
		} else {
			switch res.Context.(type) {
			case *sendContext:
				c := res.Context.(*sendContext)
				c.cb(&res, c)
			case *recvContext:
				res.Context.(*recvContext).cb(&res)
			}
		}
	}
}

func (this *SocketService) createAIOConn(conn net.Conn) (*goaio.AIOConn, error) {
	idx := rand.Int() % len(this.services)
	c, err := this.services[idx].CreateAIOConn(conn, goaio.AIOConnOption{
		ShareBuff: this.shareBuffer,
	})
	return c, err
}

func (this *SocketService) Close() {
	for i, _ := range this.services {
		this.services[i].Close()
	}
}

type ServiceOption struct {
	PollerCount              int
	WorkerPerPoller          int
	CompleteRoutinePerPoller int
	ShareBuffer              goaio.ShareBuffer
}

func NewSocketService(o ServiceOption) *SocketService {
	s := &SocketService{
		shareBuffer: o.ShareBuffer,
	}

	if o.PollerCount == 0 {
		o.PollerCount = 1
	}

	if o.WorkerPerPoller == 0 {
		o.WorkerPerPoller = 1
	}

	if o.CompleteRoutinePerPoller == 0 {
		o.CompleteRoutinePerPoller = 1
	}

	for i := 0; i < o.PollerCount; i++ {
		se := goaio.NewAIOService(o.WorkerPerPoller)
		s.services = append(s.services, se)
		for j := 0; j < o.CompleteRoutinePerPoller; j++ {
			go s.completeRoutine(se)
		}
	}

	return s
}

type AioInBoundProcessor interface {
	kendynet.InBoundProcessor
	GetRecvBuff() []byte
	OnData(buff []byte)
	OnSocketClose()
}

type defaultInBoundProcessor struct {
	bytes  int
	buffer []byte
}

func (this *defaultInBoundProcessor) GetRecvBuff() []byte {
	return this.buffer
}

func (this *defaultInBoundProcessor) Unpack() (interface{}, error) {
	if 0 == this.bytes {
		return nil, nil
	} else {
		msg := make([]byte, 0, this.bytes)
		msg = append(msg, this.buffer[:this.bytes]...)
		this.bytes = 0
		return msg, nil
	}
}

func (this *defaultInBoundProcessor) OnData(buff []byte) {
	this.bytes = len(buff)
}

func (this *defaultInBoundProcessor) OnSocketClose() {

}

const (
	fclosed  = uint32(1 << 1)
	frclosed = uint32(1 << 2)
	fdoclose = uint32(1 << 3)
)

type Socket struct {
	ud               atomic.Value
	sendQueue        *socket.SendQueue
	flag             util.Flag
	ioCount          int32
	aioConn          *goaio.AIOConn
	encoder          kendynet.EnCoder
	inboundProcessor AioInBoundProcessor
	errorCallback    func(kendynet.StreamSession, error)
	closeCallBack    func(kendynet.StreamSession, error)
	inboundCallBack  func(kendynet.StreamSession, interface{})
	beginOnce        sync.Once
	closeOnce        sync.Once
	doCloseOnce      sync.Once
	sendLock         int32
	sendContext      sendContext
	recvContext      recvContext
	sendOverChan     chan struct{}
	netconn          net.Conn
	closeReason      error
	swaped           []interface{}
	sendTimeout      int64
	recvTimeout      int64
}

func (s *Socket) IsClosed() bool {
	return s.flag.AtomicTest(fclosed)
}

func (s *Socket) SetEncoder(e kendynet.EnCoder) kendynet.StreamSession {
	s.encoder = e
	return s
}

func (s *Socket) SetSendQueueSize(size int) kendynet.StreamSession {
	s.sendQueue.SetFullSize(size)
	return s
}

func (this *Socket) SetRecvTimeout(timeout time.Duration) kendynet.StreamSession {
	atomic.StoreInt64(&this.recvTimeout, int64(timeout))
	return this
}

func (this *Socket) SetSendTimeout(timeout time.Duration) kendynet.StreamSession {
	atomic.StoreInt64(&this.sendTimeout, int64(timeout))
	return this
}

func (this *Socket) getRecvTimeout() time.Duration {
	return time.Duration(atomic.LoadInt64(&this.recvTimeout))
}

func (this *Socket) getSendTimeout() time.Duration {
	return time.Duration(atomic.LoadInt64(&this.sendTimeout))
}

func (s *Socket) SetErrorCallBack(cb func(kendynet.StreamSession, error)) kendynet.StreamSession {
	s.errorCallback = cb
	return s
}

func (s *Socket) SetCloseCallBack(cb func(kendynet.StreamSession, error)) kendynet.StreamSession {
	s.closeCallBack = cb
	return s
}

func (s *Socket) SetUserData(ud interface{}) kendynet.StreamSession {
	s.ud.Store(ud)
	return s
}

func (s *Socket) GetUserData() interface{} {
	return s.ud.Load()
}

func (s *Socket) GetUnderConn() interface{} {
	return s.aioConn
}

func (s *Socket) LocalAddr() net.Addr {
	return s.netconn.LocalAddr()
}

func (s *Socket) RemoteAddr() net.Addr {
	return s.netconn.RemoteAddr()
}

func (s *Socket) SetInBoundProcessor(in kendynet.InBoundProcessor) kendynet.StreamSession {
	s.inboundProcessor = in.(AioInBoundProcessor)
	return s
}

func (s *Socket) onRecvComplete(r *goaio.AIOResult) {
	if s.flag.AtomicTest(fclosed | frclosed) {
		s.ioDone()
	} else {
		recvAgain := false
		if nil != r.Err {
			if r.Err == goaio.ErrRecvTimeout {
				r.Err = kendynet.ErrRecvTimeout
				recvAgain = true
			} else {
				s.flag.AtomicSet(frclosed)
			}

			if nil != s.errorCallback {
				s.errorCallback(s, r.Err)
			} else {
				s.Close(r.Err, 0)
			}

		} else {
			s.inboundProcessor.OnData(r.Buff[:r.Bytestransfer])
			for !s.flag.AtomicTest(fclosed | frclosed) {
				msg, err := s.inboundProcessor.Unpack()
				if nil != err {
					s.Close(err, 0)
					if nil != s.errorCallback {
						s.errorCallback(s, err)
					}
					break
				} else if nil != msg {
					s.inboundCallBack(s, msg)
				} else {
					recvAgain = true
					break
				}
			}
		}

		if !recvAgain || s.flag.AtomicTest(fclosed|frclosed) || nil != s.aioConn.Recv(&s.recvContext, s.inboundProcessor.GetRecvBuff(), s.getRecvTimeout()) {
			s.ioDone()
		}
	}
}

func (s *Socket) doSend(sendContext *sendContext) {

	if nil == sendContext {
		sendContext = &s.sendContext
		sendContext.b = buffer.Get()
	}

	if sendContext.b.Len() == 0 {
		_, s.swaped = s.sendQueue.Get(s.swaped)

		for i := 0; i < len(s.swaped); i++ {
			l := sendContext.b.Len()
			switch s.swaped[i].(type) {
			case []byte:
				sendContext.b.AppendBytes(s.swaped[i].([]byte))
			default:
				if err := s.encoder.EnCode(s.swaped[i], sendContext.b); nil != err {
					//EnCode错误，这个包已经写入到b中的内容需要直接丢弃
					sendContext.b.SetLen(l)
					kendynet.GetLogger().Errorf("encode error:%v", err)
				}
			}
			s.swaped[i] = nil
		}
	}

	if sendContext.b.Len() == 0 {
		s.onSendComplete(&goaio.AIOResult{}, sendContext)
	} else if nil != s.aioConn.Send(sendContext, sendContext.b.Bytes(), s.getSendTimeout()) {
		s.onSendComplete(&goaio.AIOResult{Err: kendynet.ErrSocketClose}, sendContext)
	}
}

func (s *Socket) Send(o interface{}) error {
	if o == nil {
		return kendynet.ErrInvaildObject
	} else if _, ok := o.([]byte); !ok && nil == s.encoder {
		return kendynet.ErrInvaildEncoder
	} else {
		//send:1
		if err := s.sendQueue.Add(o); nil != err {
			if err == socket.ErrQueueFull {
				err = kendynet.ErrSendQueFull
			} else if err == socket.ErrAddTimeout {
				err = kendynet.ErrSendTimeout
			} else {
				err = kendynet.ErrSocketClose
			}
			return err
		}

		//send:2
		if atomic.CompareAndSwapInt32(&s.sendLock, 0, 1) {
			//send:3
			s.addIO()
			sendRoutinePool.Go(func() {
				s.doSend(nil)
			})
		}

		return nil
	}
}

func (s *Socket) SendWithTimeout(o interface{}, timeout time.Duration) error {
	if o == nil {
		return kendynet.ErrInvaildObject
	} else if _, ok := o.([]byte); !ok && nil == s.encoder {
		return kendynet.ErrInvaildEncoder
	} else {
		//send:1
		if err := s.sendQueue.AddWithTimeout(o, timeout); nil != err {
			if err == socket.ErrQueueFull {
				err = kendynet.ErrSendQueFull
			} else if err == socket.ErrAddTimeout {
				err = kendynet.ErrSendTimeout
			} else {
				err = kendynet.ErrSocketClose
			}
			return err
		}

		//send:2
		if atomic.CompareAndSwapInt32(&s.sendLock, 0, 1) {
			//send:3
			s.addIO()
			sendRoutinePool.Go(func() {
				s.doSend(nil)
			})
		}

		return nil
	}
}

func (s *Socket) DirectSend(bytes []byte, timeout ...time.Duration) (int, error) {
	var ttimeout time.Duration
	if len(timeout) > 0 {
		ttimeout = timeout[0]
	}

	var n int
	var err error

	ch := make(chan struct{})

	scontext := &sendContext{
		cb: func(res *goaio.AIOResult, _ *sendContext) {
			n = res.Bytestransfer
			err = res.Err
			if err == goaio.ErrSendTimeout {
				err = kendynet.ErrSendTimeout
			}
			close(ch)
		},
	}

	if nil != s.aioConn.Send(scontext, bytes, ttimeout) {
		return 0, kendynet.ErrSocketClose
	}

	<-ch

	return n, err

}

func (s *Socket) onSendComplete(r *goaio.AIOResult, sendContext *sendContext) {
	if nil == r.Err {
		if s.sendQueue.Empty() {
			//onSendComplete:1
			atomic.StoreInt32(&s.sendLock, 0)
			//onSendComplete:2
			if !s.sendQueue.Empty() && atomic.CompareAndSwapInt32(&s.sendLock, 0, 1) {
				/*
				 * 如果a routine执行到onSendComplete:1处暂停
				 * 此时b routine执行到send:2
				 *
				 * 现在有两种情况
				 *
				 * 情况1
				 * b routine先执行完后面的代码，此时sendLock==1,因此 b不会执行send:3里面的代码
				 * a 恢复执行，发现!s.sendQueue.Empty() && atomic.CompareAndSwapInt32(&s.sendLock, 0, 1) == true
				 * 由a继续触发sendRoutinePool.GoTask(s)
				 *
				 * 情况2
				 *
				 * a routine执行到onSendComplete:2暂停
				 * b routine继续执行，此时sendLock==0，b执行send:3里面的代码
				 * a 恢复执行，发现!s.sendQueue.Empty()但是,atomic.CompareAndSwapInt32(&s.sendLock, 0, 1)失败,执行onSendComplete:3
				 *
				 */
				sendContext.b.Reset()
				s.doSend(sendContext)
				return
			} else {
				//onSendComplete:3
				if s.sendQueue.Closed() {
					s.netconn.(interface{ CloseWrite() error }).CloseWrite()
					close(s.sendOverChan)
				}
			}
		} else {
			sendContext.b.Reset()
			s.doSend(sendContext)
			return
		}
	} else if !s.flag.AtomicTest(fclosed) {

		if r.Err == goaio.ErrSendTimeout {
			r.Err = kendynet.ErrSendTimeout
		}

		if nil != s.errorCallback {
			if r.Err != kendynet.ErrSendTimeout {
				close(s.sendOverChan)
				s.Close(r.Err, 0)
				s.errorCallback(s, r.Err)
			} else {
				s.errorCallback(s, r.Err)
				//如果是发送超时且用户没有关闭socket,再次请求发送
				if !s.flag.AtomicTest(fclosed) {
					//超时可能会发送部分数据
					sendContext.b.DropFirstNBytes(r.Bytestransfer)
					s.doSend(sendContext)
					return
				} else {
					close(s.sendOverChan)
				}
			}
		} else {
			close(s.sendOverChan)
			s.Close(r.Err, 0)
		}
	}

	sendContext.b.Free()

	s.ioDone()

}

func (s *Socket) ShutdownRead() {
	s.flag.AtomicSet(frclosed)
	s.netconn.(interface{ CloseRead() error }).CloseRead()
}

func (s *Socket) ShutdownWrite() {
	closeOK, remain := s.sendQueue.Close()
	if closeOK && remain == 0 && atomic.LoadInt32(&s.sendLock) == 0 {
		s.netconn.(interface{ CloseWrite() error }).CloseWrite()
	}
}

func (s *Socket) BeginRecv(cb func(kendynet.StreamSession, interface{})) (err error) {
	s.beginOnce.Do(func() {
		if nil == cb {
			err = errors.New("BeginRecv cb is nil")
		} else if s.flag.AtomicTest(fclosed | frclosed) {
			err = kendynet.ErrSocketClose
		} else {
			if nil == s.inboundProcessor {
				s.inboundProcessor = &defaultInBoundProcessor{
					buffer: make([]byte, 4096),
				}
			}
			s.inboundCallBack = cb
			s.addIO()
			if err = s.aioConn.Recv(&s.recvContext, s.inboundProcessor.GetRecvBuff(), s.getRecvTimeout()); nil != err {
				s.ioDone()
			}
		}
	})
	return
}

func (s *Socket) addIO() {
	atomic.AddInt32(&s.ioCount, 1)
}

func (s *Socket) ioDone() {
	if atomic.AddInt32(&s.ioCount, -1) == 0 && s.flag.AtomicTest(fdoclose) {
		s.doCloseOnce.Do(func() {
			if nil != s.inboundProcessor {
				s.inboundProcessor.OnSocketClose()
			}
			if nil != s.closeCallBack {
				s.closeCallBack(s, s.closeReason)
			}
		})
	}
}

func (s *Socket) Close(reason error, delay time.Duration) {
	s.closeOnce.Do(func() {
		runtime.SetFinalizer(s, nil)
		s.flag.AtomicSet(fclosed)
		_, remain := s.sendQueue.Close()
		if remain > 0 && delay > 0 {
			s.ShutdownRead()
			ticker := time.NewTicker(delay)
			go func() {
				select {
				case <-s.sendOverChan:
				case <-ticker.C:
				}

				ticker.Stop()
				s.aioConn.Close(nil)
			}()
		} else {
			s.aioConn.Close(nil)
		}

		s.closeReason = reason
		s.flag.AtomicSet(fdoclose)

		if atomic.LoadInt32(&s.ioCount) == 0 {
			s.doCloseOnce.Do(func() {
				if nil != s.inboundProcessor {
					s.inboundProcessor.OnSocketClose()
				}
				if nil != s.closeCallBack {
					s.closeCallBack(s, reason)
				}
			})
		}
	})
}

func NewSocket(service *SocketService, netConn net.Conn) kendynet.StreamSession {

	s := &Socket{}
	c, err := service.createAIOConn(netConn)
	if err != nil {
		return nil
	}
	s.aioConn = c
	s.sendQueue = socket.NewSendQueue(128)
	s.netconn = netConn
	s.sendOverChan = make(chan struct{})
	s.sendContext.cb = func(r *goaio.AIOResult, c *sendContext) {
		s.onSendComplete(r, c)
	}

	s.recvContext.cb = func(r *goaio.AIOResult) {
		s.onRecvComplete(r)
	}

	runtime.SetFinalizer(s, func(s *Socket) {
		s.Close(errors.New("gc"), 0)
	})

	return s
}

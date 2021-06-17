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

func (this *SocketService) completeRoutine(s *goaio.AIOService) {
	for {
		res, ok := s.GetCompleteStatus()
		if !ok {
			break
		} else {
			res.Context.(func(*goaio.AIOResult))(&res)
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
	fwclosed = uint32(1 << 3)
	fdoclose = uint32(1 << 4)
)

type Socket struct {
	ud               atomic.Value
	muW              sync.Mutex
	sendQueue        *socket.SendQueue //sendqueue //*list.List
	flag             util.Flag
	aioConn          *goaio.AIOConn
	encoder          kendynet.EnCoder
	inboundProcessor AioInBoundProcessor
	errorCallback    func(kendynet.StreamSession, error)
	closeCallBack    func(kendynet.StreamSession, error)
	inboundCallBack  func(kendynet.StreamSession, interface{})
	beginOnce        sync.Once
	closeOnce        sync.Once
	doCloseOnce      sync.Once
	sendLock         bool
	b                *buffer.Buffer
	sendOverChan     chan struct{}
	netconn          net.Conn
	sendCB           func(*goaio.AIOResult)
	recvCB           func(*goaio.AIOResult)
	closeReason      error
	ioCount          int32
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

		if !recvAgain || s.flag.AtomicTest(fclosed|frclosed) || nil != s.aioConn.Recv(s.recvCB, s.inboundProcessor.GetRecvBuff(), s.getRecvTimeout()) {
			s.ioDone()
		}
	}
}

/*
 *  实现gopool.Task接口,避免无谓的闭包创建
 */

func (s *Socket) Do() {
	s.doSend()
}

func (s *Socket) doSend() {

	//只有之前请求的buff全部发送完毕才填充新的buff
	if nil == s.b {
		s.b = buffer.Get()
	}

	_, s.swaped = s.sendQueue.Get(s.swaped)

	for i := 0; i < len(s.swaped); i++ {
		l := s.b.Len()
		switch s.swaped[i].(type) {
		case []byte:
			s.b.AppendBytes(s.swaped[i].([]byte))
		default:
			if err := s.encoder.EnCode(s.swaped[i], s.b); nil != err {
				//EnCode错误，这个包已经写入到b中的内容需要直接丢弃
				s.b.SetLen(l)
				kendynet.GetLogger().Errorf("encode error:%v", err)
			}
		}
		s.swaped[i] = nil
	}

	if s.b.Len() == 0 {
		s.onSendComplete(&goaio.AIOResult{})
	} else if nil != s.aioConn.Send(s.sendCB, s.b.Bytes(), s.getSendTimeout()) {
		s.onSendComplete(&goaio.AIOResult{Err: kendynet.ErrSocketClose})
	}
}

func (s *Socket) releaseb() {
	if nil != s.b {
		s.b.Free()
		s.b = nil
	}
}

func (s *Socket) onSendComplete(r *goaio.AIOResult) {
	defer s.ioDone()
	if nil == r.Err {
		s.muW.Lock()
		//发送完成释放发送buff
		if s.sendQueue.Empty() {
			s.releaseb()
			s.sendLock = false
			if s.flag.AtomicTest(fwclosed) {
				s.netconn.(interface{ CloseWrite() error }).CloseWrite()
				close(s.sendOverChan)
			}
			s.muW.Unlock()
		} else {
			s.muW.Unlock()
			s.b.Reset()
			s.addIO()
			sendRoutinePool.GoTask(s)
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
				s.releaseb()
			} else {
				s.errorCallback(s, r.Err)
				//如果是发送超时且用户没有关闭socket,再次请求发送
				if !s.flag.AtomicTest(fclosed) {
					//超时可能会发送部分数据
					s.b.DropFirstNBytes(r.Bytestransfer)
					s.addIO()
					sendRoutinePool.GoTask(s)
				} else {
					close(s.sendOverChan)
					s.releaseb()
				}
			}
		} else {
			close(s.sendOverChan)
			s.Close(r.Err, 0)
			s.releaseb()
		}
	} else {
		s.releaseb()
	}
}

/*
 *  应该避免在外来数据的cb中使用可能阻塞的参数调用Send,因为cb是从completeRoutine直接回调上来的，如果Send阻塞将导致
 *  completeRoutine阻塞
 */

func (s *Socket) Send(o interface{}, timeout ...time.Duration) error {
	if o == nil {
		return kendynet.ErrInvaildObject
	} else if _, ok := o.([]byte); !ok && nil == s.encoder {
		return kendynet.ErrInvaildEncoder
	} else {
		if len(timeout) == 0 {
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
		} else {
			var ttimeout time.Duration
			if len(timeout) > 0 {
				ttimeout = timeout[0]
			}

			if err := s.sendQueue.AddWithTimeout(o, ttimeout); nil != err {
				if err == socket.ErrQueueFull {
					err = kendynet.ErrSendQueFull
				} else if err == socket.ErrAddTimeout {
					err = kendynet.ErrSendTimeout
				} else {
					err = kendynet.ErrSocketClose
				}
				return err
			}
		}

		s.muW.Lock()
		if !s.sendLock {
			s.addIO()
			s.sendLock = true
			s.muW.Unlock()
			sendRoutinePool.GoTask(s)
		} else {
			s.muW.Unlock()
		}
		return nil
	}
}

func (s *Socket) DirectSend(bytes []byte, timeout ...time.Duration) (int, error) {
	if s.flag.AtomicTest(fwclosed | fclosed) {
		return 0, kendynet.ErrSocketClose
	}

	var ttimeout time.Duration
	if timeout[0] > 0 {
		ttimeout = timeout[0]
	}

	var n int
	var err error

	ch := make(chan struct{})

	sendCompleteCB := func(res *goaio.AIOResult) {
		n = res.Bytestransfer
		err = res.Err
		if err == goaio.ErrSendTimeout {
			err = kendynet.ErrSendTimeout
		}
		close(ch)
	}

	if nil != s.aioConn.Send(sendCompleteCB, bytes, ttimeout) {
		return 0, kendynet.ErrSocketClose
	}

	<-ch

	return n, err

}

func (s *Socket) ShutdownRead() {
	s.flag.AtomicSet(frclosed)
	s.netconn.(interface{ CloseRead() error }).CloseRead()
}

func (s *Socket) ShutdownWrite() {
	if s.flag.AtomicTest(fwclosed | fclosed) {
		return
	} else {
		s.flag.AtomicSet(fwclosed)
		s.muW.Lock()
		defer s.muW.Unlock()
		if s.sendQueue.Empty() && !s.sendLock {
			s.netconn.(interface{ CloseWrite() error }).CloseWrite()
		}
	}
}

func (s *Socket) BeginRecv(cb func(kendynet.StreamSession, interface{})) (err error) {
	s.beginOnce.Do(func() {

		if nil == cb {
			err = errors.New("BeginRecv cb is nil")
		} else {

			s.addIO()

			if s.flag.AtomicTest(fclosed | frclosed) {
				s.ioDone()
				err = kendynet.ErrSocketClose
			} else {
				//发起第一个recv
				if nil == s.inboundProcessor {
					s.inboundProcessor = &defaultInBoundProcessor{
						buffer: make([]byte, 4096),
					}
				}
				s.inboundCallBack = cb
				if err = s.aioConn.Recv(s.recvCB, s.inboundProcessor.GetRecvBuff(), s.getRecvTimeout()); nil != err {
					s.ioDone()
				}
			}
		}
	})
	return
}

func (s *Socket) addIO() {
	atomic.AddInt32(&s.ioCount, 1)
}

func (s *Socket) ioDone() {
	if 0 == atomic.AddInt32(&s.ioCount, -1) && s.flag.AtomicTest(fdoclose) {
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

		s.sendQueue.Close()
		s.flag.AtomicSet(fclosed)

		if !s.flag.AtomicTest(fwclosed) && delay > 0 {
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
	s.sendQueue = socket.NewSendQueue(1024)
	s.netconn = netConn
	s.sendOverChan = make(chan struct{})

	s.sendCB = func(r *goaio.AIOResult) {
		s.onSendComplete(r)
	}

	s.recvCB = func(r *goaio.AIOResult) {
		s.onRecvComplete(r)
	}

	runtime.SetFinalizer(s, func(s *Socket) {
		s.Close(errors.New("gc"), 0)
	})

	return s
}

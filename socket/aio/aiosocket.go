package aio

import (
	"container/list"
	"errors"
	"github.com/sniperHW/goaio"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/sniperHW/kendynet/gopool"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type SocketService struct {
	services          []*goaio.AIOService
	outboundTaskQueue []*goaio.TaskQueue
	shareBuffer       goaio.ShareBuffer
}

type ioContext struct {
	s *Socket
	t rune
}

func (this *SocketService) completeRoutine(s *goaio.AIOService) {
	for {
		res, err := s.GetCompleteStatus()
		if nil != err {
			break
		} else {
			context := res.Context.(*ioContext)
			if context.t == 'r' {
				context.s.onRecvComplete(&res)
			} else {
				context.s.onSendComplete(&res)
			}
		}
	}
}

func (this *SocketService) bind(conn net.Conn) (*goaio.AIOConn, *goaio.TaskQueue, error) {
	idx := rand.Int() % len(this.services)
	c, err := this.services[idx].Bind(conn, goaio.AIOConnOption{
		SendqueSize: 1,
		RecvqueSize: 1,
		ShareBuff:   this.shareBuffer,
	})
	return c, this.outboundTaskQueue[idx], err
}

func (this *SocketService) outboundRoutine(tq *goaio.TaskQueue) {
	for {
		var err error
		queue := make([]interface{}, 0, 512)
		for {
			queue, err = tq.Pop(queue)
			if nil != err {
				return
			} else {
				for _, v := range queue {
					v.(*Socket).doSend()
				}
			}
		}
	}
}

func (this *SocketService) Close() {
	for i, _ := range this.services {
		this.services[i].Close()
		this.outboundTaskQueue[i].Close()
	}
}

func NewSocketService(shareBuffer goaio.ShareBuffer) *SocketService {
	s := &SocketService{
		shareBuffer: shareBuffer,
	}

	for i := 0; i < 2; i++ {
		se := goaio.NewAIOService(2)
		tq := goaio.NewTaskQueue()
		s.services = append(s.services, se)
		s.outboundTaskQueue = append(s.outboundTaskQueue, tq)
		go s.completeRoutine(se)
		go s.outboundRoutine(tq)
	}

	return s
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

func (this *defaultInBoundProcessor) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{}, error) {
	return nil, nil
}

const (
	fclosed  = int32(1 << 1)
	frclosed = int32(1 << 2)
)

type Socket struct {
	ud               atomic.Value
	muW              sync.Mutex
	sendQueue        *list.List
	flag             int32
	aioConn          *goaio.AIOConn
	encoder          kendynet.EnCoder
	inboundProcessor kendynet.InBoundProcessor
	errorCallback    func(kendynet.StreamSession, error)
	closeCallBack    func(kendynet.StreamSession, error)
	inboundCallBack  func(kendynet.StreamSession, interface{})
	beginOnce        sync.Once
	closeOnce        sync.Once
	sendQueueSize    int
	sendLock         bool
	b                *buffer.Buffer
	offset           int
	ioWait           sync.WaitGroup
	sendOverChan     chan struct{}
	netconn          net.Conn
	tq               *goaio.TaskQueue
	sendContext      ioContext
	recvContext      ioContext
}

func (s *Socket) IsClosed() bool {
	return s.testFlag(fclosed)
}

func (s *Socket) setFlag(flag int32) {
	for {
		f := atomic.LoadInt32(&s.flag)
		if atomic.CompareAndSwapInt32(&s.flag, f, f|flag) {
			break
		}
	}
}

func (s *Socket) testFlag(flag int32) bool {
	return atomic.LoadInt32(&s.flag)&flag > 0
}

func (s *Socket) SetEncoder(e kendynet.EnCoder) kendynet.StreamSession {
	s.encoder = e
	return s
}

func (s *Socket) SetSendQueueSize(size int) kendynet.StreamSession {
	s.muW.Lock()
	defer s.muW.Unlock()
	s.sendQueueSize = size
	return s
}

func (s *Socket) SetRecvTimeout(timeout time.Duration) kendynet.StreamSession {
	s.aioConn.SetRecvTimeout(timeout)
	return s
}

func (s *Socket) SetSendTimeout(timeout time.Duration) kendynet.StreamSession {
	s.aioConn.SetSendTimeout(timeout)
	return s
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
	return s.netconn
}

func (s *Socket) GetNetConn() net.Conn {
	return s.netconn
}

func (s *Socket) LocalAddr() net.Addr {
	return s.netconn.LocalAddr()
}

func (s *Socket) RemoteAddr() net.Addr {
	return s.netconn.RemoteAddr()
}

func (s *Socket) SetInBoundProcessor(in kendynet.InBoundProcessor) kendynet.StreamSession {
	s.inboundProcessor = in
	return s
}

func (s *Socket) getDefaultInboundProcessor() kendynet.InBoundProcessor {
	return &defaultInBoundProcessor{
		buffer: make([]byte, 4096),
	}
}

func (s *Socket) onRecvComplete(r *goaio.AIOResult) {
	if s.testFlag(fclosed | frclosed) {
		s.ioWait.Done()
	} else {
		recvAgain := false

		defer func() {
			if !s.testFlag(fclosed|frclosed) && recvAgain {
				b := s.inboundProcessor.GetRecvBuff()
				if nil != s.aioConn.Recv(b, &s.recvContext) {
					s.ioWait.Done()
				}
			} else {
				s.ioWait.Done()
			}
		}()

		if nil != r.Err {

			if r.Err == goaio.ErrRecvTimeout {
				r.Err = kendynet.ErrRecvTimeout
			}

			if nil != s.errorCallback {
				if r.Err == kendynet.ErrRecvTimeout {
					recvAgain = true
				} else {
					s.setFlag(frclosed)
				}
				s.errorCallback(s, r.Err)
			} else {
				s.Close(r.Err, 0)
			}

		} else {
			//fmt.Println("Bytestransfer", r.Bytestransfer)
			s.inboundProcessor.OnData(r.Buff[:r.Bytestransfer])
			for !s.testFlag(fclosed | frclosed) {
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
	}
}

func (s *Socket) emitSendTask() {
	s.ioWait.Add(1)
	if nil == s.tq.Push(s) {
		s.sendLock = true
	} else {
		s.ioWait.Done()
		s.sendLock = false
	}
}

func (s *Socket) doSend() {
	const maxsendsize = kendynet.SendBufferSize

	var err error

	s.muW.Lock()
	//只有之前请求的buff全部发送完毕才填充新的buff
	if nil == s.b {
		s.b = buffer.Get()
		for v := s.sendQueue.Front(); v != nil; v = s.sendQueue.Front() {
			s.sendQueue.Remove(v)
			if err = s.encoder.EnCode(v.Value, s.b); nil != err {
				s.b.Free()
				s.b = nil
				break
			} else if s.b.Len() >= maxsendsize {
				break
			}
		}
		s.offset = 0
	}

	s.muW.Unlock()

	if nil == err {
		if nil != s.aioConn.Send(s.b.Bytes()[s.offset:], &s.sendContext) {
			s.ioWait.Done()
		}
	} else {
		s.ioWait.Done()
		if !s.testFlag(fclosed) {
			if nil != s.errorCallback {
				s.Close(err, 0)
				s.errorCallback(s, err)
			} else {
				s.Close(err, 0)
			}
		}
	}
}

func (s *Socket) onSendComplete(r *goaio.AIOResult) {
	defer s.ioWait.Done()
	if nil == r.Err {
		s.muW.Lock()
		defer s.muW.Unlock()
		//发送完成释放发送buff
		s.b.Free()
		s.b = nil
		if s.sendQueue.Len() == 0 {
			s.sendLock = false
			if s.testFlag(fclosed) {
				close(s.sendOverChan)
			}
		} else {
			s.emitSendTask()
		}
	} else if !s.testFlag(fclosed) {

		if r.Err == goaio.ErrSendTimeout {
			r.Err = kendynet.ErrSendTimeout
			//超时可能会发送部分数据
			s.offset += r.Bytestransfer
		}

		if nil != s.errorCallback {
			if r.Err != kendynet.ErrSendTimeout {
				s.Close(r.Err, 0)
			}
			s.errorCallback(s, r.Err)

			//如果是发送超时且用户没有关闭socket,再次请求发送
			if r.Err == kendynet.ErrSendTimeout && !s.testFlag(fclosed) {
				s.muW.Lock()
				s.emitSendTask()
				s.muW.Unlock()
			}

		} else {
			s.Close(r.Err, 0)
		}
	}
}

func (s *Socket) Send(o interface{}) error {
	if s.encoder == nil {
		return kendynet.ErrInvaildEncoder
	} else if nil == o {
		return kendynet.ErrInvaildObject
	} else if s.testFlag(fclosed) {
		return kendynet.ErrSocketClose
	} else {
		s.muW.Lock()
		defer s.muW.Unlock()

		if s.sendQueue.Len() > s.sendQueueSize {
			return kendynet.ErrSendQueFull
		}

		s.sendQueue.PushBack(o)

		if !s.sendLock {
			s.emitSendTask()
		}
		return nil
	}
}

func (s *Socket) ShutdownRead() {
	s.setFlag(frclosed)
	s.netconn.(interface{ CloseRead() error }).CloseRead()
}

func (s *Socket) BeginRecv(cb func(kendynet.StreamSession, interface{})) (err error) {
	s.beginOnce.Do(func() {
		if nil == cb {
			panic("BeginRecv cb is nil")
		}

		if s.testFlag(fclosed | frclosed) {
			err = kendynet.ErrSocketClose
		} else {
			//发起第一个recv
			if nil == s.inboundProcessor {
				s.inboundProcessor = s.getDefaultInboundProcessor()
			}
			s.inboundCallBack = cb

			s.ioWait.Add(1)
			if err = s.aioConn.Recv(s.inboundProcessor.GetRecvBuff(), &s.recvContext); nil != err {
				s.ioWait.Done()
			}

		}
	})
	return
}

func (s *Socket) Close(reason error, delay time.Duration) {
	s.closeOnce.Do(func() {
		runtime.SetFinalizer(s, nil)

		s.setFlag(fclosed)

		s.muW.Lock()
		if s.sendQueue.Len() > 0 {
			delay = delay * time.Second
		} else {
			delay = 0
		}
		s.muW.Unlock()

		if delay > 0 {
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

		gopool.Go(func() {
			s.ioWait.Wait()
			if nil != s.inboundProcessor {
				s.inboundProcessor.OnSocketClose()
			}
			if nil != s.closeCallBack {
				s.closeCallBack(s, reason)
			}
		})
	})
}

func NewSocket(service *SocketService, netConn net.Conn) kendynet.StreamSession {

	s := &Socket{}
	c, tq, err := service.bind(netConn)
	if err != nil {
		return nil
	}
	s.tq = tq
	s.aioConn = c
	s.sendQueueSize = 256
	s.sendQueue = list.New()
	s.netconn = netConn
	s.sendOverChan = make(chan struct{})
	s.sendContext = ioContext{s: s, t: 's'}
	s.recvContext = ioContext{s: s, t: 'r'}

	runtime.SetFinalizer(s, func(s *Socket) {
		//fmt.Println("gc")
		s.Close(errors.New("gc"), 0)
	})

	return s
}

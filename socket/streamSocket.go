/*
*  tcp或unix域套接字会话
 */

package socket

import (
	"errors"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"net"
	"runtime"
	"time"
)

type StreamSocketInBoundProcessor interface {
	kendynet.InBoundProcessor
	GetRecvBuff() []byte
	OnData([]byte)
}

type defaultSSInBoundProcessor struct {
	buffer []byte
	w      int
}

func (this *defaultSSInBoundProcessor) GetRecvBuff() []byte {
	return this.buffer[this.w:]
}

func (this *defaultSSInBoundProcessor) OnData(data []byte) {
	this.w += len(data)
}

func (this *defaultSSInBoundProcessor) Unpack() (interface{}, error) {
	if this.w == 0 {
		return nil, nil
	} else {
		o := make([]byte, 0, this.w)
		o = append(o, this.buffer[:this.w]...)
		this.w = 0
		return o, nil
	}
}

type StreamSocket struct {
	SocketBase
	inboundProcessor StreamSocketInBoundProcessor
	conn             net.Conn
}

func (this *StreamSocket) getInBoundProcessor() kendynet.InBoundProcessor {
	return this.inboundProcessor
}

func (this *StreamSocket) SetInBoundProcessor(in kendynet.InBoundProcessor) kendynet.StreamSession {
	this.inboundProcessor = in.(StreamSocketInBoundProcessor)
	return this
}

func (this *StreamSocket) DirectSend(bytes []byte, timeout ...time.Duration) (int, error) {
	if this.flag.AtomicTest(fclosed | frclosed) {
		return 0, kendynet.ErrSocketClose
	} else {
		var ttimeout time.Duration
		if len(timeout) > 0 {
			ttimeout = timeout[0]
		}

		var n int
		var err error

		if ttimeout > 0 {
			this.conn.SetWriteDeadline(time.Now().Add(ttimeout))
			n, err = this.conn.Write(bytes)
			this.conn.SetWriteDeadline(time.Time{})
		} else {
			n, err = this.conn.Write(bytes)
		}

		if kendynet.IsNetTimeout(err) {
			err = kendynet.ErrSendTimeout
		}

		return n, err
	}
}

func (this *StreamSocket) recvThreadFunc() {
	defer this.ioDone()

	oldTimeout := this.getRecvTimeout()
	timeout := oldTimeout

	for !this.flag.AtomicTest(fclosed | frclosed) {

		var (
			p   interface{}
			err error
			n   int
		)

		isUnpackError := false

		for {
			p, err = this.inboundProcessor.Unpack()
			if nil != p {
				break
			} else if nil != err {
				isUnpackError = true
				break
			} else {

				oldTimeout = timeout
				timeout = this.getRecvTimeout()

				if oldTimeout != timeout && timeout == 0 {
					this.conn.SetReadDeadline(time.Time{})
				}

				buff := this.inboundProcessor.GetRecvBuff()
				if timeout > 0 {
					this.conn.SetReadDeadline(time.Now().Add(timeout))
					n, err = this.conn.Read(buff)
				} else {
					n, err = this.conn.Read(buff)
				}

				if nil == err {
					this.inboundProcessor.OnData(buff[:n])
				} else {
					break
				}
			}
		}

		if !this.flag.AtomicTest(fclosed | frclosed) {
			if nil != err {
				if kendynet.IsNetTimeout(err) {
					err = kendynet.ErrRecvTimeout
				}
				if nil != this.errorCallback {
					if isUnpackError {
						this.Close(err, 0)
					} else if err != kendynet.ErrRecvTimeout {
						this.flag.AtomicSet(frclosed)
					}

					this.errorCallback(this, err)
				} else {
					this.Close(err, 0)
				}
			} else if p != nil {
				this.inboundCallBack(this, p)
			}
		} else {
			break
		}
	}
}

func (this *StreamSocket) sendThreadFunc() {
	defer func() {
		close(this.sendCloseChan)
		this.ioDone()
	}()

	var err error

	localList := make([]interface{}, 0, 32)

	closed := false

	var n int

	oldTimeout := this.getSendTimeout()
	timeout := oldTimeout

	for {

		closed, localList = this.sendQue.Get(localList)
		size := len(localList)
		if closed && size == 0 {
			this.conn.(interface{ CloseWrite() error }).CloseWrite()
			break
		}

		b := buffer.Get()
		for i := 0; i < size; {
			if b.Len() == 0 {
				for i < size {
					l := b.Len()

					switch localList[i].(type) {
					case []byte:
						b.AppendBytes(localList[i].([]byte))
					default:
						err = this.encoder.EnCode(localList[i], b)
					}
					localList[i] = nil
					i++
					if nil != err {
						//EnCode错误，这个包已经写入到b中的内容需要直接丢弃
						b.SetLen(l)
						kendynet.GetLogger().Errorf("encode error:%v", err)
					}
				}
			}

			if b.Len() == 0 {
				break
			}

			oldTimeout = timeout
			timeout = this.getSendTimeout()

			if oldTimeout != timeout && timeout == 0 {
				this.conn.SetWriteDeadline(time.Time{})
			}

			if timeout > 0 {
				this.conn.SetWriteDeadline(time.Now().Add(timeout))
				n, err = this.conn.Write(b.Bytes())
			} else {
				n, err = this.conn.Write(b.Bytes())
			}

			if nil == err {
				b.Reset()
			} else if !this.flag.AtomicTest(fclosed) {
				if kendynet.IsNetTimeout(err) {
					err = kendynet.ErrSendTimeout
				} else {
					this.Close(err, 0)
				}

				if nil != this.errorCallback {
					this.errorCallback(this, err)
				}

				if this.flag.AtomicTest(fclosed) {
					break
				} else {
					//超时可能完成部分发送，将已经发送部分丢弃
					b.DropFirstNBytes(n)
				}
			} else {
				break
			}
		}
		b.Free()
	}
}

func NewStreamSocket(conn net.Conn) kendynet.StreamSession {
	switch conn.(type) {
	case *net.TCPConn, *net.UnixConn:
		break
	default:
		return nil
	}

	s := &StreamSocket{
		conn: conn,
	}
	s.SocketBase = SocketBase{
		sendQue:       NewSendQueue(128),
		sendCloseChan: make(chan struct{}),
		imp:           s,
	}

	runtime.SetFinalizer(s, func(s *StreamSocket) {
		s.Close(errors.New("gc"), 0)
	})

	return s
}

func (this *StreamSocket) getNetConn() net.Conn {
	return this.conn
}

func (this *StreamSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *StreamSocket) defaultInBoundProcessor() kendynet.InBoundProcessor {
	return &defaultSSInBoundProcessor{buffer: make([]byte, 4096)}
}

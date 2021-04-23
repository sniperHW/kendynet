/*
*  tcp或unix域套接字会话
 */

package socket

import (
	"errors"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/sniperHW/kendynet/util"
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

func (this *StreamSocket) recvThreadFunc() {
	defer this.ioWait.Done()

	oldTimeout := this.getRecvTimeout()
	timeout := oldTimeout

	for !this.testFlag(fclosed | frclosed) {

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

		if !this.testFlag(fclosed | frclosed) {
			if nil != err {
				if kendynet.IsNetTimeout(err) {
					err = kendynet.ErrRecvTimeout
				}

				if nil != this.errorCallback {

					if isUnpackError {
						this.Close(err, 0)
					} else if err != kendynet.ErrRecvTimeout {
						this.setFlag(frclosed)
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
	defer this.ioWait.Done()

	var err error

	localList := make([]interface{}, 0, 32)

	closed := false

	var i int

	var size int

	const maxsendsize = kendynet.SendBufferSize

	var b *buffer.Buffer

	var offset int

	var n int

	defer func() {
		if nil != b {
			b.Free()
		}
	}()

	oldTimeout := this.getSendTimeout()
	timeout := oldTimeout

	for {

		if i >= size {
			closed, localList = this.sendQue.Swap(localList)
			size = len(localList)
			if closed && size == 0 {
				this.conn.(interface{ CloseWrite() error }).CloseWrite()
				break
			}
			i = 0
		}

		if nil == b {
			b = buffer.Get()
			for i < size {
				err = this.encoder.EnCode(localList[i], b)
				localList[i] = nil
				i++
				if nil != err {
					b.Free()
					b = nil
					if !this.testFlag(fclosed) {
						this.Close(err, 0)
						if nil != this.errorCallback {
							this.errorCallback(this, err)
						}
					}
					return
				} else if b.Len() >= maxsendsize {
					break
				}
			}
			offset = 0
		}

		oldTimeout = timeout
		timeout = this.getSendTimeout()

		if oldTimeout != timeout && timeout == 0 {
			this.conn.SetWriteDeadline(time.Time{})
		}

		if timeout > 0 {
			this.conn.SetWriteDeadline(time.Now().Add(timeout))
			n, err = this.conn.Write(b.Bytes()[offset:])
		} else {
			n, err = this.conn.Write(b.Bytes()[offset:])
		}

		offset += n

		if nil == err {
			b.Free()
			b = nil
		} else if !this.testFlag(fclosed) {
			if kendynet.IsNetTimeout(err) {
				err = kendynet.ErrSendTimeout
			} else {
				this.Close(err, 0)
			}

			if nil != this.errorCallback {
				this.errorCallback(this, err)
			}

			if this.testFlag(fclosed) {
				return
			}
		} else {
			return
		}
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
		sendQue:       util.NewBlockQueue(1024),
		sendCloseChan: make(chan struct{}),
		imp:           s,
	}

	runtime.SetFinalizer(s, func(s *StreamSocket) {
		s.Close(errors.New("gc"), 0)
	})

	return s
}

func (this *StreamSocket) Read(b []byte) (int, error) {
	return this.conn.Read(b)
}

func (this *StreamSocket) GetNetConn() net.Conn {
	return this.conn
}

func (this *StreamSocket) GetUnderConn() interface{} {
	return this.GetNetConn()
}

func (this *StreamSocket) defaultInBoundProcessor() kendynet.InBoundProcessor {
	return &defaultSSInBoundProcessor{buffer: make([]byte, 4096)}
}

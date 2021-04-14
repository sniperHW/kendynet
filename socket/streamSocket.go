/*
*  tcp或unix域套接字会话
 */

package socket

import (
	//"bufio"
	//"fmt"
	"errors"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/sniperHW/kendynet/util"
	"net"
	"runtime"
	"time"
)

type defaultSSInBoundProcessor struct {
	buffer []byte
}

func (this *defaultSSInBoundProcessor) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{}, error) {
	n, err := sess.(*StreamSocket).Read(this.buffer[:])
	if err != nil {
		return nil, err
	}

	msg := make([]byte, 0, n)
	msg = append(msg, this.buffer[:n]...)
	return msg, err
}

func (this *defaultSSInBoundProcessor) GetRecvBuff() []byte {
	return nil
}

func (this *defaultSSInBoundProcessor) OnData([]byte) {

}

func (this *defaultSSInBoundProcessor) Unpack() (interface{}, error) {
	return nil, nil
}

func (this *defaultSSInBoundProcessor) OnSocketClose() {

}

type StreamSocket struct {
	SocketBase
	conn net.Conn
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
						if nil != this.errorCallback {
							this.Close(err, 0)
							this.errorCallback(this, err)
						} else {
							this.Close(err, 0)
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
			}

			if nil != this.errorCallback {
				if err != kendynet.ErrSendTimeout {
					this.Close(err, 0)
				}
				this.errorCallback(this, err)
			} else {
				this.Close(err, 0)
			}

			if this.testFlag(fclosed) {
				return
			}
		} else {
			return
		}
	}
}

/*func (this *StreamSocket) sendThreadFunc() {
	defer this.ioWait.Done()

	var err error

	writer := bufio.NewWriterSize(this.conn, kendynet.SendBufferSize)

	localList := make([]interface{}, 0, 32)

	closed := false

	for {

		timeout := this.getSendTimeout()

		closed, localList = this.sendQue.Swap(localList)
		size := len(localList)
		if closed && size == 0 {
			break
		}

		for i := 0; i < size; i++ {
			msg := localList[i].(kendynet.Message)
			localList[i] = nil

			data := msg.Bytes()
			for data != nil || (i == (size-1) && writer.Buffered() > 0) {
				if data != nil {
					var s int
					if len(data) > writer.Available() {
						s = writer.Available()
					} else {
						s = len(data)
					}
					writer.Write(data[:s])

					if s != len(data) {
						data = data[s:]
					} else {
						data = nil
					}
				}

				if writer.Available() == 0 || i == (size-1) {
					if timeout > 0 {
						this.conn.SetWriteDeadline(time.Now().Add(timeout))
						err = writer.Flush()
						this.conn.SetWriteDeadline(time.Time{})
					} else {
						err = writer.Flush()
					}

					if err != nil {
						if !this.testFlag(fclosed) {
							if kendynet.IsNetTimeout(err) {
								err = kendynet.ErrSendTimeout
							}

							if nil != this.errorCallback {
								if err != kendynet.ErrSendTimeout {
									this.Close(err, 0)
								}
								this.errorCallback(this, err)
							} else {
								this.Close(err, 0)
							}

							if this.testFlag(fclosed) {
								return
							}
						} else {
							return
						}
					}
				}
			}
		}
	}
}*/

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
		//fmt.Println("gc")
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

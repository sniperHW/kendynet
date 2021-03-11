/*
*  websocket会话
 */

package socket

import (
	"fmt"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/message"
	"github.com/sniperHW/kendynet/util"
	"net"
	//"sync/atomic"
	"errors"
	"github.com/sniperHW/kendynet/buffer"
	"runtime"
	"time"
)

var ErrInvaildWSMessage = fmt.Errorf("invaild websocket message")

/*
*   无封包结构，直接将收到的所有数据返回
 */

type defaultWSInBoundProcessor struct {
}

func (this *defaultWSInBoundProcessor) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{}, error) {
	mt, msg, err := sess.(*WebSocket).Read()
	if err != nil {
		return nil, err
	} else {
		return message.NewWSMessage(mt, msg), nil
	}
}

func (this *defaultWSInBoundProcessor) GetRecvBuff() []byte {
	return nil
}

func (this *defaultWSInBoundProcessor) OnData([]byte) {

}

func (this *defaultWSInBoundProcessor) Unpack() (interface{}, error) {
	return nil, nil
}

func (this *defaultWSInBoundProcessor) OnSocketClose() {

}

type WebSocket struct {
	SocketBase
	conn *gorilla.Conn
}

func (this *WebSocket) sendThreadFunc() {
	defer this.ioWait.Done()

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
			var err error
			msg, ok := localList[i].(*message.WSMessage)
			if !ok {
				panic("invaild ws message")
			}
			localList[i] = nil
			var buff []byte
			var b *buffer.Buffer
			if nil != msg.Data() {
				b = buffer.Get()
				if err = this.encoder.EnCode(msg.Data(), b); nil != err {
					if !this.testFlag(fclosed) {
						if nil != this.errorCallback {
							this.Close(err, 0)
							this.errorCallback(this, err)
						} else {
							this.Close(err, 0)
						}
					}
					return
				} else {
					buff = b.Bytes()
				}
			}

			if timeout > 0 {
				this.conn.SetWriteDeadline(time.Now().Add(timeout))
				err = this.conn.WriteMessage(msg.Type(), buff)
				this.conn.SetWriteDeadline(time.Time{})
			} else {
				err = this.conn.WriteMessage(msg.Type(), buff)
			}

			if nil != b {
				b.Free()
			}

			if err != nil && !this.testFlag(fclosed) {

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
			}
		}
	}
}

func NewWSSocket(conn *gorilla.Conn) kendynet.StreamSession {
	if nil == conn {
		return nil
	} else {

		conn.SetCloseHandler(func(code int, text string) error {
			conn.UnderlyingConn().Close()
			return nil
		})

		s := &WebSocket{
			conn: conn,
		}
		s.SocketBase = SocketBase{
			sendQue:       util.NewBlockQueue(1024),
			sendCloseChan: make(chan struct{}),
			imp:           s,
		}

		runtime.SetFinalizer(s, func(s *WebSocket) {
			s.Close(errors.New("gc"), 0)
		})

		return s
	}
}

func (this *WebSocket) SetPingHandler(h func(appData string) error) {
	this.conn.SetPingHandler(h)
}

func (this *WebSocket) SetPongHandler(h func(appData string) error) {
	this.conn.SetPongHandler(h)
}

func (this *WebSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *WebSocket) GetNetConn() net.Conn {
	return this.conn.UnderlyingConn()
}

func (this *WebSocket) Read() (messageType int, p []byte, err error) {
	return this.conn.ReadMessage()
}

func (this *WebSocket) defaultInBoundProcessor() kendynet.InBoundProcessor {
	return &defaultWSInBoundProcessor{}
}

func (this *WebSocket) SendWSClose(reason string) error {
	return this.Send(message.NewWSMessage(message.WSCloseMessage, gorilla.FormatCloseMessage(1000, reason)))
}

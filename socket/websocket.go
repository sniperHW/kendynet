/*
*  websocket会话
 */

package socket

import (
	"errors"
	"fmt"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/sniperHW/kendynet/message"
	"github.com/sniperHW/kendynet/util"
	"net"
	"runtime"
	"time"
)

var ErrInvaildWSMessage = fmt.Errorf("invaild websocket message")

/*
 *   无封包结构，直接将收到的所有数据返回
 */

type WebsocketInBoundProcessor interface {
	kendynet.InBoundProcessor
	OnData(int, []byte)
}

type defaultWSInBoundProcessor struct {
	gotData     bool
	messageType int
	data        []byte
}

func (this *defaultWSInBoundProcessor) OnData(messageType int, data []byte) {
	this.gotData = true
	this.messageType = messageType
	this.data = data
}

func (this *defaultWSInBoundProcessor) Unpack() (interface{}, error) {
	if this.gotData {
		msg := message.NewWSMessage(this.messageType, this.data)
		this.gotData = false
		this.data = nil
		return msg, nil
	} else {
		return nil, nil
	}
}

type WebSocket struct {
	SocketBase
	inboundProcessor WebsocketInBoundProcessor
	conn             *gorilla.Conn
}

func (this *WebSocket) getInBoundProcessor() kendynet.InBoundProcessor {
	return this.inboundProcessor
}

func (this *WebSocket) SetInBoundProcessor(in kendynet.InBoundProcessor) kendynet.StreamSession {
	this.inboundProcessor = in.(WebsocketInBoundProcessor)
	return this
}

func (this *WebSocket) recvThreadFunc() {
	defer this.ioWait.Done()

	oldTimeout := this.getRecvTimeout()
	timeout := oldTimeout

	for !this.testFlag(fclosed | frclosed) {

		var (
			p           interface{}
			err         error
			messageType int
			data        []byte
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

				if timeout > 0 {
					this.conn.SetReadDeadline(time.Now().Add(timeout))
					messageType, data, err = this.conn.ReadMessage()
				} else {
					messageType, data, err = this.conn.ReadMessage()
				}

				if nil == err {
					this.inboundProcessor.OnData(messageType, data)
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

func (this *WebSocket) sendThreadFunc() {
	defer this.ioWait.Done()

	localList := make([]interface{}, 0, 32)

	closed := false

	oldTimeout := this.getSendTimeout()
	timeout := oldTimeout

	for {

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
						this.Close(err, 0)
						if nil != this.errorCallback {
							this.errorCallback(this, err)
						}
					}
					return
				} else {
					buff = b.Bytes()
				}
			}

			oldTimeout = timeout
			timeout = this.getSendTimeout()

			if oldTimeout != timeout && timeout == 0 {
				this.conn.SetWriteDeadline(time.Time{})
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
				} else {
					this.Close(err, 0)
				}

				if nil != this.errorCallback {
					this.errorCallback(this, err)
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

func (this *WebSocket) defaultInBoundProcessor() kendynet.InBoundProcessor {
	return &defaultWSInBoundProcessor{}
}

func (this *WebSocket) SendWSClose(reason string) error {
	return this.Send(message.NewWSMessage(message.WSCloseMessage, gorilla.FormatCloseMessage(1000, reason)))
}

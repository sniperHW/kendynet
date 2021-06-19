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

func (this *WebSocket) DirectSend(bytes []byte, timeout ...time.Duration) (int, error) {
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
			err = this.conn.WriteMessage(message.WSBinaryMessage, bytes)
			this.conn.SetWriteDeadline(time.Time{})
		} else {
			err = this.conn.WriteMessage(message.WSBinaryMessage, bytes)
		}

		if nil == err {
			n = len(bytes)
		} else if kendynet.IsNetTimeout(err) {
			err = kendynet.ErrSendTimeout
		}

		return n, err
	}
}

func (this *WebSocket) recvThreadFunc() {
	defer this.ioDone()

	oldTimeout := this.getRecvTimeout()
	timeout := oldTimeout

	for !this.flag.AtomicTest(fclosed | frclosed) {

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

func (this *WebSocket) sendThreadFunc() {

	defer func() {
		close(this.sendCloseChan)
		this.ioDone()
	}()

	localList := make([]interface{}, 0, 32)

	closed := false

	oldTimeout := this.getSendTimeout()
	timeout := oldTimeout

	for {

		closed, localList = this.sendQue.Get(localList)
		size := len(localList)
		if closed && size == 0 {
			this.conn.UnderlyingConn().(interface{ CloseWrite() error }).CloseWrite()
			break
		}

		b := buffer.Get()
		for i := 0; i < size; i++ {
			var err error

			var msgType int

			switch localList[i].(type) {
			case []byte:
				b.AppendBytes(localList[i].([]byte))
				msgType = message.WSBinaryMessage
			case *message.WSMessage:
				msg := localList[i].(*message.WSMessage)
				msgType = msg.Type()
				if nil != msg.Data() {
					b = buffer.Get()
					if err = this.encoder.EnCode(msg.Data(), b); nil != err {
						kendynet.GetLogger().Errorf("encode error:%v", err)
						b.Reset()
						continue
					}
				}
			default:
				panic("invaild message")
			}

			localList[i] = nil

			oldTimeout = timeout
			timeout = this.getSendTimeout()

			if oldTimeout != timeout && timeout == 0 {
				this.conn.SetWriteDeadline(time.Time{})
			}

			if timeout > 0 {
				this.conn.SetWriteDeadline(time.Now().Add(timeout))
				err = this.conn.WriteMessage(msgType, b.Bytes())
				this.conn.SetWriteDeadline(time.Time{})
			} else {
				err = this.conn.WriteMessage(msgType, b.Bytes())
			}

			b.Reset()

			if err != nil && !this.flag.AtomicTest(fclosed) {

				if kendynet.IsNetTimeout(err) {
					err = kendynet.ErrSendTimeout
				} else {
					this.Close(err, 0)
				}

				if nil != this.errorCallback {
					this.errorCallback(this, err)
				}

				if this.flag.AtomicTest(fclosed) {
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
			sendQue:       NewSendQueue(128),
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

func (this *WebSocket) getNetConn() net.Conn {
	return this.conn.UnderlyingConn()
}

func (this *WebSocket) defaultInBoundProcessor() kendynet.InBoundProcessor {
	return &defaultWSInBoundProcessor{}
}

func (this *WebSocket) SendWSClose(reason string) error {
	return this.Send(message.NewWSMessage(message.WSCloseMessage, gorilla.FormatCloseMessage(1000, reason)))
}

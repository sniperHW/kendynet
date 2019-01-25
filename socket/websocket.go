/*
*  websocket会话
 */

package socket

import (
	"fmt"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"net"
	//"sync"
	"time"
)

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	WSTextMessage = 1

	// BinaryMessage denotes a binary data message.
	WSBinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	WSCloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	WSPingMessage = 9

	// PongMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	WSPongMessage = 10
)

var ErrInvaildWSMessage = fmt.Errorf("invaild websocket message")

/*
 *  WSMessage与普通的ByteBuffer Msg的区别在于多了一个messageType字段
 */
type WSMessage struct {
	messageType int
	buff        *kendynet.ByteBuffer
}

func (this *WSMessage) Bytes() []byte {
	return this.buff.Bytes()
}

func (this *WSMessage) PutBytes(idx uint64, value []byte) error {
	return this.buff.PutBytes(idx, value)
}

func (this *WSMessage) GetBytes(idx uint64, size uint64) ([]byte, error) {
	return this.buff.GetBytes(idx, size)
}

func (this *WSMessage) PutString(idx uint64, value string) error {
	return this.buff.PutString(idx, value)
}

func (this *WSMessage) GetString(idx uint64, size uint64) (string, error) {
	return this.buff.GetString(idx, size)
}

func (this *WSMessage) Type() int {
	return this.messageType
}

func NewMessage(messageType int, optional ...interface{}) *WSMessage {
	buff := kendynet.NewByteBuffer(optional...)
	if nil == buff {
		fmt.Printf("nil == buff\n")
		return nil
	}
	return &WSMessage{messageType: messageType, buff: buff}
}

type WebSocket struct {
	*SocketBase
	conn *gorilla.Conn
}

func (this *WebSocket) sendMessage(msg kendynet.Message) error {
	if msg == nil {
		return kendynet.ErrInvaildBuff
	} else if (this.flag&closed) > 0 || (this.flag&wclosed) > 0 {
		return kendynet.ErrSocketClose
	} else {
		switch msg.(type) {
		case *WSMessage:
			if nil == msg.(*WSMessage) {
				return ErrInvaildWSMessage
			}
			fullReturn := true
			err := this.sendQue.AddNoWait(msg, fullReturn)
			if nil != err {
				if err == util.ErrQueueClosed {
					err = kendynet.ErrSocketClose
				} else if err == util.ErrQueueFull {
					err = kendynet.ErrSendQueFull
				}
				return err
			}
			break
		default:
			return ErrInvaildWSMessage
		}
	}
	return nil
}

func (this *WebSocket) recvThreadFunc() {

	var p interface{}
	var err error

	for !this.isClosed() {
		recvTimeout := this.recvTimeout
		if recvTimeout > 0 {
			this.conn.SetReadDeadline(time.Now().Add(recvTimeout))
			p, err = this.receiver.ReceiveAndUnpack(this)
			this.conn.SetReadDeadline(time.Time{})
		} else {
			p, err = this.receiver.ReceiveAndUnpack(this)
		}

		if this.isClosed() {
			break
		}

		if err != nil || p != nil {
			var event kendynet.Event
			event.Session = this
			if err != nil {
				event.EventType = kendynet.EventTypeError
				event.Data = err
				if !kendynet.IsNetTimeout(err) {
					kendynet.Errorf("ReceiveAndUnpack error:%s\n", err.Error())
					this.mutex.Lock()
					this.flag |= (rclosed | wclosed)
					this.mutex.Unlock()
				}
			} else {
				event.EventType = kendynet.EventTypeMessage
				event.Data = p
			}
			/*出现错误不主动退出循环，除非用户调用了session.Close()
			 * 避免用户遗漏调用Close(不调用Close会持续通告错误)
			 */
			this.onEvent(&event)
		}
	}
}

func (this *WebSocket) sendThreadFunc() {
	for {
		closed, localList := this.sendQue.Get()
		size := len(localList)
		if closed && size == 0 {
			break
		}

		for i := 0; i < size; i++ {
			var err error
			msg := localList[i].(*WSMessage)
			timeout := this.sendTimeout
			if msg.messageType == WSBinaryMessage || msg.messageType == WSTextMessage {
				if timeout > 0 {
					this.conn.SetWriteDeadline(time.Now().Add(timeout))
					err = this.conn.WriteMessage(msg.messageType, msg.Bytes())
					this.conn.SetWriteDeadline(time.Time{})
				} else {
					err = this.conn.WriteMessage(msg.messageType, msg.Bytes())
				}

			} else if msg.messageType == WSCloseMessage || msg.messageType == WSPingMessage || msg.messageType == WSPingMessage {
				var deadline time.Time
				if timeout > 0 {
					deadline = time.Now().Add(timeout)
				}
				err = this.conn.WriteControl(msg.messageType, msg.Bytes(), deadline)
			}

			if err != nil && msg.messageType != WSCloseMessage {
				if this.sendQue.Closed() {
					return
				}

				if kendynet.IsNetTimeout(err) {
					err = kendynet.ErrSendTimeout
				} else {
					kendynet.Errorf("websocket write error:%s\n", err.Error())
					this.mutex.Lock()
					this.flag |= wclosed
					this.mutex.Unlock()
				}

				event := &kendynet.Event{Session: this, EventType: kendynet.EventTypeError, Data: err}
				this.onEvent(event)
			}
		}
	}
	this.sendCloseChan <- 1
}

func (this *WebSocket) Close(reason string, delay time.Duration) {
	this.mutex.Lock()
	if (this.flag & closed) > 0 {
		this.mutex.Unlock()
		return
	}

	this.closeReason = reason
	this.flag |= (closed | rclosed)
	if (this.flag & wclosed) > 0 {
		delay = 0 //写端已经关闭忽略delay参数
	} else {
		delay = delay * time.Second
	}

	if delay > 0 {
		this.shutdownRead()
		message := gorilla.FormatCloseMessage(1000, reason)
		this.sendQue.AddNoWait(NewMessage(WSCloseMessage, message))
		this.sendQue.Close()
		ticker := time.NewTicker(delay)
		if (this.flag & started) == 0 {
			go this.sendThreadFunc()
		}
		this.mutex.Unlock()
		go func() {
			select {
			case <-this.sendCloseChan:
			case <-ticker.C:
			}
			ticker.Stop()
			this.doClose()
			return
		}()
	} else {
		this.sendQue.Close()
		this.mutex.Unlock()
		this.doClose()
		return
	}
}

func NewWSSocket(conn *gorilla.Conn, sendQueueSize ...int) kendynet.StreamSession {
	if nil == conn {
		return nil
	} else {
		conn.SetCloseHandler(func(code int, text string) error {
			return fmt.Errorf("peer close reason[%s]", text)
		})

		s := &WebSocket{
			conn:       conn,
			SocketBase: &SocketBase{},
		}
		s.sendQue = util.NewBlockQueue(sendQueueSize...)
		s.sendCloseChan = make(chan int, 1)
		s.imp = s
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

func (this *WebSocket) getSocketConn() net.Conn {
	return this.conn.UnderlyingConn()
}

func (this *WebSocket) Read() (messageType int, p []byte, err error) {
	return this.conn.ReadMessage()
}

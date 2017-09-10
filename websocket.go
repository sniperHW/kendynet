/*
*  websocket会话
*/

package kendynet

/*
import (
	   "net"
	   "fmt"
	   "time"
	   "sync"
	   "bufio"
	   "io"	
	   "unsafe"  
	   "github.com/gorilla/websocket"
)

type WebSocket struct {
	conn websocket.Conn
	ud   interface{}
}
*/

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage = 1

	// BinaryMessage denotes a binary data message.
	BinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

/*
*  WSMessage与普通的ByteBuffer Msg的区别在于多了一个messageType字段   
*/
type WSMessage struct {
	messageType int
	buff       *ByteBuffer
}

func (this *WSMessage) Bytes() []byte {
	return this.buff.Bytes()
}

func (this *WSMessage) PutBytes(idx uint64,value []byte)(error){
	return this.buff.PutBytes(idx,value)
}

func (this *WSMessage) GetBytes(idx uint64,size uint64) ([]byte,error) {
	return this.buff.GetBytes(idx,size)
}

func (this *WSMessage) PutString(idx uint64,value string)(error){
	return this.buff.PutString(idx,value)
}

func (this *WSMessage) GetString(idx uint64,size uint64) (string,error) {
	return this.buff.GetString(idx,size)
}

func NewWSMessage(messageType int,optional ...interface{}) *WSMessage {
	buff := NewByteBuffer(optional)
	if nil == buff {
		return nil
	}
	return &WSMessage{messageType:messageType,buff:buff}
}

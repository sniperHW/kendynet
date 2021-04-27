package message

//import (
//"fmt"
//"github.com/sniperHW/kendynet"
//)

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

/*
 *  WSMessage与普通的ByteBuffer Msg的区别在于多了一个messageType字段
 */
type WSMessage struct {
	messageType int
	data        interface{}
}

func (this *WSMessage) Type() int {
	return this.messageType
}

func (this *WSMessage) Data() interface{} {
	return this.data
}

func NewWSMessage(messageType int, data ...interface{}) *WSMessage {
	switch messageType {
	case WSTextMessage, WSBinaryMessage, WSCloseMessage, WSPingMessage, WSPongMessage:
	default:
		return nil
	}

	if len(data) > 0 {
		return &WSMessage{messageType: messageType, data: data[0]}
	} else {
		return &WSMessage{messageType: messageType}
	}
}

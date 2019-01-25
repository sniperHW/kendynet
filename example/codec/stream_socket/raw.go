package stream_socket

/*
*   无封包结构，直接将收到的所有数据返回
 */

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket"
)

const (
	minBuffSize = 4096
)

type RawReceiver struct {
	buffer []byte
}

func (this *RawReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{}, error) {
	n, err := sess.(*socket.StreamSocket).Read(this.buffer[:])
	if err != nil {
		return nil, err
	}
	msg := kendynet.NewByteBuffer(n)
	msg.AppendBytes(this.buffer[:n])
	return msg, err
}

func NewRawReceiver(buffsize uint64) *RawReceiver {
	if buffsize < minBuffSize {
		buffsize = minBuffSize
	}
	return &RawReceiver{buffer: make([]byte, buffsize)}
}

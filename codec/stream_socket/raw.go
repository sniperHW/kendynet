package stream_socket

/*
*   无封包结构，直接将收到的所有数据返回
*/

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/stream_socket"
)

const (
	minBuffSize = 4096
	minSizeRemain = 256
)

type RawReceiver struct {
	buffsize  uint64
	space     uint64
	buffer    [] byte
}

func (this *RawReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{},error) {
	
	//如果缓冲区小于minSizeRemain字节，重新分配缓冲区
	if this.space < minSizeRemain {
		this.buffer = make([]byte,this.buffsize)
		this.space = this.buffsize
	}

	idx := this.buffsize - this.space

	n,err := sess.(*stream_socket.StreamSocket).Read(this.buffer[idx:])
	if err != nil {
		return nil,err
	}

	if n <= 0 {
		return nil,nil
	}

	this.space -= uint64(n)

	return kendynet.NewByteBuffer(this.buffer[idx:idx+uint64(n)],(uint64)(n)),nil		
}


func NewRawReceiver(buffsize uint64)(*RawReceiver){
	if buffsize < minBuffSize {
		buffsize = minBuffSize
	}
	return &RawReceiver{buffsize:buffsize,space:0}
}

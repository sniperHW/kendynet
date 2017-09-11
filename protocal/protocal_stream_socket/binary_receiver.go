package protocal_stream_socket

import (
	"github.com/sniperHW/kendynet"
	//"fmt"
)

const (
	minBuffSize = 4096
	minSizeRemain = 256
)

type StreamSocketBinaryReceiver struct {
	buffsize  uint64
	space     uint64
	buffer    [] byte
}

func (this *StreamSocketBinaryReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{},error) {
	
	//如果缓冲区小于minSizeRemain字节，重新分配缓冲区
	if this.space < minSizeRemain {
		this.buffer = make([]byte,this.buffsize)
		this.space = this.buffsize
	}

	idx := this.buffsize - this.space

	n,err := sess.(*kendynet.StreamSocket).Read(this.buffer[idx:])
	if err != nil {
		return nil,err
	}

	if n <= 0 {
		return nil,nil
	}

	this.space -= uint64(n)

	return kendynet.NewByteBuffer(this.buffer[idx:idx+uint64(n)],(uint64)(n)),nil		
}


func NewBinaryReceiver(buffsize uint64)(*StreamSocketBinaryReceiver){
	if buffsize < minBuffSize {
		buffsize = minBuffSize
	}
	return &StreamSocketBinaryReceiver{buffsize:buffsize,space:0}
}

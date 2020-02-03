package codec

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/socket/aio"
)

const minBuffSize = 4096

type PbEncoder struct {
	maxMsgSize uint64
}

func NewPbEncoder(maxMsgSize uint64) *PbEncoder {
	return &PbEncoder{maxMsgSize: maxMsgSize}
}

func (this *PbEncoder) EnCode(o interface{}) (kendynet.Message, error) {
	return pb.Encode(o, this.maxMsgSize)
}

type BufferPool struct {
	pool chan []byte
}

func NewBufferPool(bytes int, count int) *BufferPool {
	p := &BufferPool{
		pool: make(chan []byte, count),
	}
	for i := 0; i < count; i++ {
		p.pool <- make([]byte, bytes)
	}
	return p
}

func (p *BufferPool) Get() []byte {
	return <-p.pool
}

func (p *BufferPool) Put(buff []byte) {
	p.pool <- buff[:cap(buff)]
}

type PBReceiver struct {
	buffer       []byte
	isPoolBuffer bool
	pool         *BufferPool
	maxMsgSize   int
	w            int
	r            int
	firstRecv    bool
}

func NewPBReceiver(pool *BufferPool, maxMsgSize int) *PBReceiver {
	receiver := &PBReceiver{}
	receiver.maxMsgSize = maxMsgSize
	receiver.pool = pool
	return receiver
}

func (this *PBReceiver) OnRecvOk(s kendynet.StreamSession, buff []byte) {
	if nil == this.buffer {
		this.buffer = buff
		this.r = 0
		this.w = len(buff)
		this.isPoolBuffer = true
	} else {
		this.isPoolBuffer = false
		space := len(this.buffer) - this.w
		if space < len(buff) {
			newBuffer := make([]byte, this.w+len(buff))
			copy(newBuffer, this.buffer[:this.w])
			this.buffer = newBuffer
		}
		copy(this.buffer[:this.w], buff)
		this.w += len(buff)
	}
	s.(*aio.AioSocket).Recv(nil)
}

func (this *PBReceiver) unPack() (interface{}, error) {
	msg, dataLen, err := pb.Decode(this.buffer, uint64(this.r), uint64(this.w), uint64(this.maxMsgSize))
	if dataLen > 0 {
		this.r += int(dataLen)
	}
	return msg, err
}

func (this *PBReceiver) ReceiveAndUnpack(s kendynet.StreamSession) (interface{}, error) {

	if !this.firstRecv {
		//由框架发起第一次recv
		s.(*aio.AioSocket).Recv(nil)
		this.firstRecv = true
		return nil, nil
	}

	if nil == this.buffer {
		return nil, nil
	}

	msg, err := this.unPack()
	if msg == nil || nil != err {
		if this.isPoolBuffer {
			this.pool.Put(this.buffer)
		}
		this.buffer = nil
		this.r = 0
		this.w = 0
	}

	if nil == msg && nil == err {
		if this.r != this.w {
			if this.isPoolBuffer {
				newBuffer := make([]byte, this.maxMsgSize+int(pb.PBHeaderSize))
				copy(newBuffer, this.buffer[this.r:this.w])
				this.buffer = newBuffer
				this.isPoolBuffer = false
			} else {
				copy(this.buffer, this.buffer[this.r:this.w])
			}
			this.w = this.w - this.r
			this.r = 0
		}
	}
	return msg, err
}

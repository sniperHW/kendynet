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
	buffer         []byte
	recvBuff       []byte
	maxpacket      int
	unpackSize     int
	unpackIdx      int
	initBuffSize   int
	totalMaxPacket int
	minBuffRemain  int
}

func NewPBReceiver(pool *BufferPool, maxMsgSize int) *PBReceiver {
	receiver := &PBReceiver{}
	//完整数据包大小为head+data
	receiver.totalMaxPacket = maxMsgSize + int(pb.PBHeaderSize)
	doubleTotalPacketSize := receiver.totalMaxPacket * 2
	if doubleTotalPacketSize < minBuffSize {
		receiver.initBuffSize = minBuffSize
	} else {
		receiver.initBuffSize = doubleTotalPacketSize
	}
	receiver.buffer = make([]byte, receiver.initBuffSize)
	receiver.recvBuff = receiver.buffer
	receiver.maxpacket = maxMsgSize
	receiver.minBuffRemain = receiver.totalMaxPacket / 4
	return receiver
}
func (this *PBReceiver) unPack() (interface{}, error) {
	msg, dataLen, err := pb.Decode(this.buffer, uint64(this.unpackIdx), uint64(this.unpackIdx+this.unpackSize), uint64(this.maxpacket))
	if dataLen > 0 {
		this.unpackIdx += int(dataLen)
		this.unpackSize -= int(dataLen)
	}
	return msg, err
}

func (this *PBReceiver) OnRecvOk(s kendynet.StreamSession, buff []byte) {
	l := len(buff)
	this.unpackSize += l
	this.recvBuff = this.recvBuff[l:]
}

func (this *PBReceiver) ReceiveAndUnpack(s kendynet.StreamSession) (interface{}, error) {
	msg, err := this.unPack()
	if nil == msg && nil == err {
		if len(this.recvBuff) < this.minBuffRemain {
			if this.unpackSize > 0 {
				//有数据尚未解包，需要移动到buffer前部
				copy(this.buffer, this.buffer[this.unpackIdx:this.unpackIdx+this.unpackSize])
			}
			this.unpackIdx = 0
			this.recvBuff = this.buffer[this.unpackSize:]
		} else if this.unpackSize == 0 {
			this.unpackIdx = 0
			this.recvBuff = this.buffer
		}
		s.(*aio.AioSocket).PostRecv(this.recvBuff)
	}
	return msg, err
}

func (this *PBReceiver) StartReceive(s kendynet.StreamSession) {
	s.(*aio.AioSocket).PostRecv(this.recvBuff)
}

func (this *PBReceiver) OnClose() {

}

/*type PBReceiver struct {
	buffer       []byte
	isPoolBuffer bool
	pool         *BufferPool
	maxMsgSize   int
	w            int
	r            int
	receiveSize  int
}

func NewPBReceiver(pool *BufferPool, maxMsgSize int) *PBReceiver {
	receiver := &PBReceiver{}
	receiver.maxMsgSize = maxMsgSize
	receiver.pool = pool
	return receiver
}

func (this *PBReceiver) OnClose() {
	if nil != this.buffer && this.isPoolBuffer {
		this.pool.Put(this.buffer)
	}
}

func (this *PBReceiver) StartReceive(s kendynet.StreamSession) {
	s.(*aio.AioSocket).PostRecv(nil)
}

func (this *PBReceiver) OnRecvOk(s kendynet.StreamSession, buff []byte) {
	this.receiveSize += len(buff)
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
		this.pool.Put(buff)
	}
}

func (this *PBReceiver) unPack() (interface{}, error) {
	msg, dataLen, err := pb.Decode(this.buffer, uint64(this.r), uint64(this.w), uint64(this.maxMsgSize))
	if dataLen > 0 {
		this.r += int(dataLen)
	}
	return msg, err
}

func (this *PBReceiver) ReceiveAndUnpack(s kendynet.StreamSession) (interface{}, error) {

	for {

		msg, err := this.unPack()

		if nil != msg {
			return msg, nil
		} else {
			if nil == err {
				if this.r != this.w {
					if this.isPoolBuffer {
						newBuffer := make([]byte, this.maxMsgSize+int(pb.PBHeaderSize))
						copy(newBuffer, this.buffer[this.r:this.w])
						this.pool.Put(this.buffer)
						this.buffer = newBuffer
						this.isPoolBuffer = false
					} else {
						copy(this.buffer, this.buffer[this.r:this.w])
					}
					this.w = this.w - this.r
					this.r = 0
				} else {
					this.w = 0
					this.r = 0
					if this.isPoolBuffer {
						this.pool.Put(this.buffer)
						this.buffer = nil
						this.isPoolBuffer = false
					}
				}

				if this.receiveSize < 65536 {
					buff, err := s.(*aio.AioSocket).Recv(nil)
					if nil != err {
						return nil, err
					} else if nil != buff {
						this.OnRecvOk(s, buff)
					} else {
						this.receiveSize = 0
						return nil, nil
					}
				} else {
					this.receiveSize = 0
					if err = s.(*aio.AioSocket).PostRecv(nil); nil != err {
						return nil, err
					}
				}

			} else {
				if this.isPoolBuffer {
					this.pool.Put(this.buffer)
					this.buffer = nil
					this.isPoolBuffer = false
				}
				return nil, err
			}

		}
	}
}*/

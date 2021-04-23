package codec

import (
	"errors"
	//"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/sniperHW/kendynet/example/pb"
	//"github.com/sniperHW/kendynet/socket"
)

func isPow2(size int) bool {
	return (size & (size - 1)) == 0
}

func sizeofPow2(size int) int {
	if isPow2(size) {
		return size
	}
	size = size - 1
	size = size | (size >> 1)
	size = size | (size >> 2)
	size = size | (size >> 4)
	size = size | (size >> 8)
	size = size | (size >> 16)
	return size + 1
}

type PbEncoder struct {
	maxMsgSize int
}

func NewPbEncoder(maxMsgSize int) *PbEncoder {
	return &PbEncoder{maxMsgSize: maxMsgSize}
}

func (this *PbEncoder) EnCode(o interface{}, b *buffer.Buffer) (e error) {
	if nil == b {
		e = errors.New("b==nil")
	} else {
		_, e = pb.Encode(o, b, this.maxMsgSize)
	}
	return
}

type PBReceiver struct {
	buffer    []byte
	w         int
	r         int
	maxpacket int
}

func NewPBReceiver(maxMsgSize int) *PBReceiver {
	return &PBReceiver{
		buffer:    make([]byte, 4096),
		maxpacket: maxMsgSize,
	}
}

func (this *PBReceiver) GetRecvBuff() []byte {
	return this.buffer[this.w:]
}

func (this *PBReceiver) OnData(data []byte) {
	this.w += len(data)
}

func (this *PBReceiver) Unpack() (interface{}, error) {
	if this.r == this.w {
		return nil, nil
	} else {
		msg, packetSize, err := pb.Decode(this.buffer, this.r, this.w, this.maxpacket)
		if nil != msg {
			this.r += packetSize
			if this.r == this.w {
				this.r = 0
				this.w = 0
			}
		} else if nil == err {
			if packetSize > cap(this.buffer) {
				buffer := make([]byte, sizeofPow2(packetSize))
				copy(buffer, this.buffer[this.r:this.w])
				this.buffer = buffer
			} else {
				//空间足够容纳下一个包，
				copy(this.buffer, this.buffer[this.r:this.w])
			}
			this.w = this.w - this.r
			this.r = 0
		}
		return msg, err
	}
}

func (this *PBReceiver) OnSocketClose() {

}

/*type PBReceiver struct {
	recvBuff       []byte
	buffer         []byte
	maxpacket      int
	unpackSize     int
	unpackIdx      int
	initBuffSize   int
	totalMaxPacket int
	lastUnpackIdx  int
}

func NewPBReceiver(maxMsgSize int) *PBReceiver {
	receiver := &PBReceiver{}
	//完整数据包大小为head+data
	receiver.totalMaxPacket = maxMsgSize + pb.PBHeaderSize
	doubleTotalPacketSize := receiver.totalMaxPacket * 2
	if doubleTotalPacketSize < minBuffSize {
		receiver.initBuffSize = minBuffSize
	} else {
		receiver.initBuffSize = doubleTotalPacketSize
	}
	receiver.buffer = make([]byte, receiver.initBuffSize)
	receiver.recvBuff = receiver.buffer
	receiver.maxpacket = maxMsgSize
	return receiver
}

func (this *PBReceiver) unPack() (interface{}, error) {
	msg, dataLen, err := pb.Decode(this.buffer, this.unpackIdx, this.unpackIdx+this.unpackSize, this.maxpacket)
	if msg != nil {
		this.unpackIdx += dataLen
		this.unpackSize -= dataLen
	}
	return msg, err
}

func (this *PBReceiver) check(buff []byte) {
	l := len(buff)
	l = l / 8
	for i := 0; i < l; i++ {
		var j int
		for j = 0; j < 8; j++ {
			if buff[i*8+j] != 0 {
				break
			}
		}
		if j == 8 {
			kendynet.GetLogger().Info(buff)
		}
	}
}

func (this *PBReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{}, error) {
	var msg interface{}
	var err error
	for {
		msg, err = this.unPack()
		if nil != msg {
			break
		}
		if err == nil {
			if this.unpackSize == 0 {
				this.unpackIdx = 0
				this.recvBuff = this.buffer
			} else if len(this.recvBuff) < this.totalMaxPacket/4 {
				if this.unpackSize > 0 {
					//有数据尚未解包，需要移动到buffer前部
					copy(this.buffer, this.buffer[this.unpackIdx:this.unpackIdx+this.unpackSize])
				}
				this.recvBuff = this.buffer[this.unpackSize:]
				this.unpackIdx = 0
			}

			n, err := sess.(*socket.StreamSocket).Read(this.recvBuff)
			if n > 0 {
				this.lastUnpackIdx = this.unpackIdx
				this.unpackSize += n //增加待解包数据
				this.recvBuff = this.recvBuff[n:]
			}
			if err != nil {
				return nil, err
			}
		} else {
			kendynet.GetLogger().Info(this.unpackIdx, this.unpackSize, this.buffer[this.lastUnpackIdx:this.unpackIdx+this.unpackSize])
			panic("err")
			break
		}
	}

	return msg, err
}

func (this *PBReceiver) GetRecvBuff() []byte {
	return this.recvBuff
}

func (this *PBReceiver) OnData(data []byte) {
	this.unpackSize += len(data)
	this.recvBuff = this.recvBuff[len(data):]
}

func (this *PBReceiver) Unpack() (msg interface{}, err error) {
	msg, err = this.unPack()
	if msg == nil && err == nil {
		if this.unpackSize == 0 {
			this.unpackIdx = 0
			this.recvBuff = this.buffer
		} else if len(this.recvBuff) < this.totalMaxPacket/4 {
			if this.unpackSize > 0 {
				//有数据尚未解包，需要移动到buffer前部
				copy(this.buffer, this.buffer[this.unpackIdx:this.unpackIdx+this.unpackSize])
			}
			this.recvBuff = this.buffer[this.unpackSize:]
			this.unpackIdx = 0
		}
	}
	return
}

func (this *PBReceiver) OnSocketClose() {

}*/

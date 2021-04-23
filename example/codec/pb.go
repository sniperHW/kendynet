package codec

import (
	"errors"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/sniperHW/kendynet/example/pb"
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

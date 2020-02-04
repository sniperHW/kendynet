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

func NewPBReceiver(maxMsgSize int) *PBReceiver {
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

	for {
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

			buff, err := s.(*aio.AioSocket).Recv(this.recvBuff)
			if nil != err {
				return nil, err
			} else if nil != buff {
				this.OnRecvOk(s, buff)
				continue
			} else {
				break
			}
		}
		return msg, err
	}
	return nil, nil
}

func (this *PBReceiver) StartReceive(s kendynet.StreamSession) {
	s.(*aio.AioSocket).PostRecv(this.recvBuff)
}

func (this *PBReceiver) OnClose() {

}

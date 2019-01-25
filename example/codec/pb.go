package codec

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/socket"
	"os"
	"time"
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
	recvBuff       []byte
	buffer         []byte
	maxpacket      uint64
	unpackSize     uint64
	unpackIdx      uint64
	initBuffSize   uint64
	totalMaxPacket uint64
}

func NewPBReceiver(maxMsgSize uint64) *PBReceiver {
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
	if dataLen > 0 {
		this.unpackIdx += dataLen
		this.unpackSize -= dataLen
	}
	return msg, err
}

func (this *PBReceiver) check(buff []byte) {
	l := len(buff)
	l = l / 4
	for i := 0; i < l; i++ {
		var j int
		for j = 0; j < 4; j++ {
			if buff[i*4+j] != 0xFF {
				break
			}
		}
		if j == 4 {
			kendynet.Infoln(buff)
			time.Sleep(time.Second)
			os.Exit(0)
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
			if uint64(len(this.recvBuff)) < this.totalMaxPacket/4 {
				if this.unpackSize > 0 {
					//有数据尚未解包，需要移动到buffer前部
					copy(this.buffer, this.buffer[this.unpackIdx:this.unpackIdx+this.unpackSize])
				}
				this.recvBuff = this.buffer[this.unpackSize:]

				for k, _ := range this.recvBuff {
					this.recvBuff[k] = 0xFF
				}

				this.unpackIdx = 0
			}

			n, err := sess.(*socket.StreamSocket).Read(this.recvBuff)
			if n > 0 {
				this.check(this.recvBuff[:n])
				this.unpackSize += uint64(n) //增加待解包数据
				this.recvBuff = this.recvBuff[n:]
			}
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}

	return msg, err
}

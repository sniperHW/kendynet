package protocal_stream_socket

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/pb"
)

type PbEncoder struct {
	maxMsgSize uint64
}

func NewPbEncoder(maxMsgSize uint64) *PbEncoder {
	return &PbEncoder{maxMsgSize:maxMsgSize}
}

func (this *PbEncoder) EnCode(o interface{}) (kendynet.Message,error) {
	return pb.Encode(o,this.maxMsgSize)
}
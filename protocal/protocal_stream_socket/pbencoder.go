package protocal_stream_socket

import (
	"github.com/sniperHW/kendynet"
	//"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/util/pb"
	//"fmt"
	//"reflect"
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
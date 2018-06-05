package main

import (
	"github.com/golang/protobuf/proto"
	"fmt"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/example/pb"
)

func main() {

	err := pb.Register(&testproto.Test{},1)

	if err != nil {
		fmt.Printf("Register error: %s\n",err.Error())
		return
	}

	o := &testproto.Test{}
	o.A = proto.String("hello")
	o.B = proto.Int32(17)

	buff,err := pb.Encode(o,1000)

	if err != nil {
		fmt.Printf("encode error: %s\n",err.Error())
		return
	}

	msg,msglen,err := pb.Decode(buff.Bytes(),0,(uint64)(len(buff.Bytes())),1000)

	if err != nil {
		fmt.Printf("decode error: %s\n",err.Error())
		return		
	}

	if msg != nil {
		fmt.Printf("msg len:%d\n",msglen)
		fmt.Printf("msg.A:%s\n",msg.(*testproto.Test).GetA())
		fmt.Printf("msg.B:%d\n",msg.(*testproto.Test).GetB())
	}

}
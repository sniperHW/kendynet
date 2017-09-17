package test_rpc

import (
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/rpc"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/pb"
	"reflect"
	"fmt"
)

//用chan实现的一个rpc通道

type Channel struct {
	name       string
	toServer   chan interface{}
	toClient   chan interface{}
}

func NewChannel(name string) *Channel{
	channel := &Channel{}
	channel.name = name
	channel.toServer = make(chan interface{},10000)
	channel.toClient = make(chan interface{},10000)
	return channel
}

func (this *Channel) SendRPCRequest(message interface {}) error {
	this.toServer <- message
	return nil
}

func (this *Channel) SendRPCResponse(message interface {}) error {
	this.toClient <- message
	return nil
}

func (this *Channel) ServerRecive() (msg interface{},ok bool) {
	msg,ok = <- this.toServer
	return
}

func (this *Channel) ClientRecive() (msg interface{},ok bool) {
	msg,ok = <- this.toClient
	return
}

func (this *Channel) Name() string {
	return this.name
}

type TestEncoder struct {
}

func (this *TestEncoder) Encode(message rpc.RPCMessage) (interface{},error) {
	if message.Type() == rpc.RPC_REQUEST {
		req := message.(*rpc.RPCRequest)
		request := &testproto.RPCRequest{}
		request.Seq = proto.Uint64(req.Seq)
		request.Method = proto.String(req.Method)
		request.NeedResp = proto.Bool(req.NeedResp)
		if req.Arg != nil {
			buff,err := pb.Encode(req.Arg,1000)
			if err != nil {
				fmt.Printf("encode error: %s\n",err.Error())
				return nil,err
			}
			request.Arg = buff.Bytes()
		}
		return request,nil
	} else {
		resp := message.(*rpc.RPCResponse)
		response := &testproto.RPCResponse{}
		response.Seq = proto.Uint64(resp.Seq)
		if resp.Err != nil {
			response.Err = proto.String(resp.Err.Error())
		}
		if resp.Ret != nil {
			buff,err := pb.Encode(resp.Ret,1000)
			if err != nil {
				fmt.Printf("encode error: %s\n",err.Error())
				return nil,err
			}
			response.Ret = buff.Bytes()
		}
		return response,nil
	}
}

type TestDecoder struct {
}

func (this *TestDecoder) Decode(o interface{}) (rpc.RPCMessage,error) {	
	switch o.(type) {
		case *testproto.RPCRequest:
			req := o.(*testproto.RPCRequest)
			request := &rpc.RPCRequest{}
			request.Seq = req.GetSeq()
			request.Method = req.GetMethod()
			request.NeedResp = req.GetNeedResp()
			if len(req.Arg) > 0 {
				var err error
				request.Arg,_,err = pb.Decode(req.Arg,0,(uint64)(len(req.Arg)),1000)
				if err != nil {
					return nil,err
				}
			}

			return request,nil
		case *testproto.RPCResponse:
			resp := o.(*testproto.RPCResponse)
			response := &rpc.RPCResponse{}
			response.Seq = resp.GetSeq()
			if resp.Err != nil {
				response.Err = fmt.Errorf(resp.GetErr())
			}
			if len(resp.Ret) > 0 {
				var err error
				response.Ret,_,err = pb.Decode(resp.Ret,0,(uint64)(len(resp.Ret)),1000)
				if err != nil {
					return nil,err
				}
			}
			return response,nil
		default:
			return nil,fmt.Errorf("invaild obj type:%s",reflect.TypeOf(o).String())
	}	
}
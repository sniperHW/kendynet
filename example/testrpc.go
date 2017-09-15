package main 

import (
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/rpc"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/pb"
	"reflect"
	"fmt"
	"sync/atomic"
	"time"
)

//用chan实现的一个rpc通道

type Channel struct {
	name       string
	toServer   chan interface{}
	toClient   chan interface{}
}

func newChannel() *Channel{
	channel := &Channel{}
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

func (this *Channel) Name() string {
	return this.name
}

type testEncoder struct {
}

func (this *testEncoder) Encode(message rpc.RPCMessage) (interface{},error) {
	if message.Type() == rpc.RPC_REQUEST {
		req := message.(*rpc.RPCRequest)
		request := &testproto.RPCRequest{}
		request.Seq = proto.Uint64(req.Seq)
		request.Method = proto.String(req.Method)
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

type testDecoder struct {
}

func (this *testDecoder) Decode(o interface{}) (rpc.RPCMessage,error) {	
	switch o.(type) {
		case *testproto.RPCRequest:
			req := o.(*testproto.RPCRequest)
			request := &rpc.RPCRequest{}
			request.Seq = req.GetSeq()
			request.Method = req.GetMethod()
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

func main() {
	pb.Register(&testproto.Hello{})
	pb.Register(&testproto.World{})

	count := int32(0)

	go func() {
		for {
			time.Sleep(time.Second)
			c := atomic.LoadInt32(&count)
			atomic.StoreInt32(&count,0)
			fmt.Printf("count:%d\n",c)			
		}
	}()

	rpcChannel := newChannel()

	RPC,_ := rpc.NewRPCManager(&testDecoder{},&testEncoder{})

	//注册服务
	RPC.RegisterService("hello",func (arg interface{})(interface{},error){
		atomic.AddInt32(&count,1)
		//hello := arg.(*testproto.Hello)
		//fmt.Printf("%s\n",hello.GetHello())
		world := &testproto.World{}

		world.World = proto.String("world")
		return world,nil
	})

	//启动服务器
	go func(){
		for {
			msg,ok := <- rpcChannel.toServer
			if !ok {
				return
			}
			RPC.OnRPCMessage(rpcChannel,msg)
		}
	}()

	//启动客户端
	go func(){
		hello := &testproto.Hello{}
		hello.Hello = proto.String("hello")

		var onResp func(ret interface{},err error)

		onResp = func(ret interface{},err error){
			if nil != ret {
				//world := ret.(*testproto.World)
				//fmt.Printf("%s\n",world.GetWorld())
				err := RPC.Call(rpcChannel,"hello",hello,onResp)
				if err != nil {
					fmt.Printf("%s\n",err.Error())
					return
				}
			} else if nil != err {
				fmt.Printf("%s\n",err.Error())
			}
		}

		//发起第一批请求

		for i := 0; i < 100; i++ {
			err := RPC.Call(rpcChannel,"hello",hello,onResp)
			if err != nil {
				fmt.Printf("%s\n",err.Error())
				return
			}
		}

		for {
			msg,ok := <-rpcChannel.toClient
			if !ok {
				return
			}
			RPC.OnRPCMessage(rpcChannel,msg)
		}
	}()

	sigStop := make(chan bool)
	_,_ = <- sigStop

}
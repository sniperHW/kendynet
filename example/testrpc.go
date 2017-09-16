package main 

import (
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/rpc"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/pb"
	"fmt"
	"sync/atomic"
	"time"
	"github.com/sniperHW/kendynet/example/test_rpc"
)

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

	rpcChannel := test_rpc.NewChannel("testchannel")

	RPC,_ := rpc.NewRPCManager(&test_rpc.TestDecoder{},&test_rpc.TestEncoder{})

	Client,_ := rpc.NewRPCClient(RPC,rpcChannel)

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
			msg,ok := rpcChannel.ServerRecive()
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
				err := Client.Call("hello",hello,onResp)
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
			err := Client.Call("hello",hello,onResp)
			if err != nil {
				fmt.Printf("%s\n",err.Error())
				return
			}
		}

		for {
			msg,ok := rpcChannel.ClientRecive()
			if !ok {
				return
			}
			RPC.OnRPCMessage(rpcChannel,msg)
		}
	}()

	sigStop := make(chan bool)
	_,_ = <- sigStop

}
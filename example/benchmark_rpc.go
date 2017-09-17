package main

import(
	"time"
	"sync/atomic"
	"strconv"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/tcp"
	codec "github.com/sniperHW/kendynet/codec/stream_socket"		
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/example/test_rpc"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/pb"
	"github.com/sniperHW/kendynet/rpc"
)

var RPC *rpc.RPCManager

func server(service string) {
	clientcount := int32(0)
	count := int32(0)

	//注册服务
	RPC.RegisterService("hello",func (arg interface{})(interface{},error){
		atomic.AddInt32(&count,1)
		world := &testproto.World{}
		world.World = proto.String("world")
		return world,nil
	})

	RPC.RegisterService("drop",func (arg interface{})(interface{},error){
		/*
		*   服务只是增加计数，把消息丢掉。
		*   如果要实现普通的消息通信可以注册一个叫message的服务，普通消息发往message服务再做消息的分发处理
		*/
		atomic.AddInt32(&count,1)
		//hello := arg.(*testproto.Hello)
		//fmt.Printf("%s\n",hello.GetHello())
		return nil,nil
	})

	go func() {
		for {
			time.Sleep(time.Second)
			tmp := atomic.LoadInt32(&count)
			atomic.StoreInt32(&count,0)
			fmt.Printf("clientcount:%d,count:%d\n",clientcount,tmp)			
		}
	}()

	server,err := tcp.NewServer("tcp4",service)
	if server != nil {
		fmt.Printf("server running on:%s\n",service)
		err = server.Start(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount,1)
			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("server client close:%s\n",reason)
				atomic.AddInt32(&clientcount,-1)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					RPC.OnRPCMessage(event.Session,event.Data)
				}
			})
			session.Start()
		})

		if nil != err {
			fmt.Printf("TcpServer start failed %s\n",err)			
		}

	} else {
		fmt.Printf("NewTcpServer failed %s\n",err)
	}
}

func client(service string,count int) {
	
	client,err := tcp.NewClient("tcp4",service)

	if err != nil {
		fmt.Printf("NewTcpClient failed:%s\n",err.Error())
		return
	}

	for i := 0; i < count ; i++ {
		session,_,err := client.Dial()
		if err != nil {
			fmt.Printf("Dial error:%s\n",err.Error())
		} else {

			Client,_ := rpc.NewRPCClient(RPC,session) 

			hello := &testproto.Hello{}
			hello.Hello = proto.String("hello")

			//定义response回调
			var onResp func(ret interface{},err error)

			onResp = func(ret interface{},err error){
				if nil != ret {
					Client.OneWayCall("drop",hello)
					err := Client.Call("hello",hello,onResp)
					if err != nil {
						fmt.Printf("%s\n",err.Error())
						return
					}
				} else if nil != err {
					fmt.Printf("%s\n",err.Error())
				}
			}

			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n",reason)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
					RPC.OnChannelDisconnected(event.Session,event.Data.(error).Error())
				} else {
					RPC.OnRPCMessage(event.Session,event.Data)
				}
			})
			session.Start()
			Client.Call("hello",hello,onResp)
		}
	}
}


func main(){
	pb.Register(&testproto.Hello{})
	pb.Register(&testproto.World{})
	pb.Register(&testproto.RPCResponse{})
	pb.Register(&testproto.RPCRequest{})
	if len(os.Args) < 3 {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}


	mode := os.Args[1]

	if !(mode == "server" || mode == "client" || mode == "both") {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}

	RPC,_ = rpc.NewRPCManager(&test_rpc.TestDecoder{},&test_rpc.TestEncoder{})

	service := os.Args[2]

	sigStop := make(chan bool)

	if mode == "server" || mode == "both" {
		go server(service)
	}

	if mode == "client" || mode == "both" {
		if len(os.Args) < 4 {
			fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
			return
		}
		connectioncount,err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Printf(err.Error())
			return
		}
		//让服务器先运行
		time.Sleep(10000000)
		go client(service,connectioncount)

	}

	_,_ = <- sigStop

	return

}



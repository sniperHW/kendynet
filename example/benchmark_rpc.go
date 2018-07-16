package main

import(
	"time"
	"sync/atomic"
	"strconv"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/example/test_rpc"
	"github.com/sniperHW/kendynet/timer"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/rpc"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/golog"
	//"math/rand"
)

var timeoutcount int32
var reqcount int32

func server(service string) {
	count := int32(0)
	total := 0
	timer.Repeat(time.Second,nil,func (_ timer.TimerID) {
		tmp := atomic.LoadInt32(&count)
		atomic.StoreInt32(&count,0)
		tmp1 := atomic.LoadInt32(&timeoutcount)
		atomic.StoreInt32(&timeoutcount,0)
		tmp2 := atomic.LoadInt32(&reqcount)
		atomic.StoreInt32(&reqcount,0)
		fmt.Printf("count:%d,timeoutcount:%d,reqcount:%d,total:%d\n",tmp,tmp1,tmp2,total)	
	})

	server := test_rpc.NewRPCServer()
	//注册服务
	server.RegisterMethod("hello",func (replyer *rpc.RPCReplyer,arg interface{}){
		atomic.AddInt32(&count,1)
		total = total + 1
		//if rand.Int() % 1000 == 0 {
			//不返回消息让客户端超时
		//} else {
			world := &testproto.World{World:proto.String("world")}
			replyer.Reply(world,nil)
		//}
	})
	server.Serve(service)
}

func client(service string,count int) {
	arg := &testproto.Hello{Hello:proto.String("hello")}
	for i := 0; i < count ; i++ {
		go func() {
			caller := test_rpc.NewCaller()
			if err := caller.Dial(service,10 * time.Second);nil != err {
				fmt.Println(err.Error())
				return
			}
			for j:=0;j < 10;j++{
				go func(){

						for {
							_,err := caller.SyncCall("hello",arg,1000)
							atomic.AddInt32(&reqcount,1)
							if nil != err {
								fmt.Printf("err:%s\n",err.Error())
							}
						}

				}()	
			}
		}()
		/*var onResp func(ret interface{},err error)
		onResp = func(ret interface{},err error){
			if nil != ret {
				err := caller.Call("hello",hello,10,onResp)
				if err != nil {
					fmt.Printf("%s\n",err.Error())
					return
				}
			} else if nil != err {
				fmt.Printf("%s\n",err.Error())
			}
		}

		err := caller.Dial(service,10 * time.Second)
		if err != nil {
			fmt.Printf("%s\n",err.Error())
		} else {
			for j := 0 ; j < 10; j++ {
				caller.Call("hello",hello,onResp)
			}
		}*/
	}
}

func main(){

	outLogger := golog.NewOutputLogger("log","kendynet",1024*1024*1000)
	kendynet.InitLogger(outLogger)
	rpc.InitLogger(outLogger)

	if len(os.Args) < 3 {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}


	mode := os.Args[1]

	if !(mode == "server" || mode == "client" || mode == "both") {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}

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
		time.Sleep(time.Second)
		go client(service,connectioncount)

	}

	_,_ = <- sigStop

	return

}



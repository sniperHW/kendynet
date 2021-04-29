package main

//go tool pprof --seconds 30 http://localhost:8899/debug/pprof/profile
//go-torch -u http://localhost:8899 -t 30
import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/aio/test_rpc"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/golog"
	"github.com/sniperHW/kendynet/rpc"
	"github.com/sniperHW/kendynet/timer"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

var timeoutcount int32
var reqcount int32

func server(service string) {

	go func() {
		http.ListenAndServe("localhost:8899", nil)
	}()

	count := int32(0)
	total := 0
	timer.Repeat(time.Second, func(_ *timer.Timer, ctx interface{}) {
		tmp := atomic.LoadInt32(&count)
		atomic.StoreInt32(&count, 0)
		tmp1 := atomic.LoadInt32(&timeoutcount)
		atomic.StoreInt32(&timeoutcount, 0)
		tmp2 := atomic.LoadInt32(&reqcount)
		atomic.StoreInt32(&reqcount, 0)
		fmt.Printf("count:%d,timeoutcount:%d,reqcount:%d,total:%d\n", tmp, tmp1, tmp2, total)
	}, nil)

	server := test_rpc.NewRPCServer()
	//注册服务
	server.RegisterMethod("hello", func(replyer *rpc.RPCReplyer, arg interface{}) {
		atomic.AddInt32(&count, 1)
		total = total + 1
		//if rand.Int() % 1000 == 0 {
		//不返回消息让客户端超时
		//} else {
		world := &testproto.World{World: proto.String("world")}
		replyer.Reply(world, nil)
		//}
	})
	server.Serve(service)
}

func testSyncCall(caller *test_rpc.Caller) {
	for j := 0; j < 10; j++ {
		go func() {
			for {
				arg := &testproto.Hello{Hello: proto.String("hello")}
				_, err := caller.Call("hello", arg, time.Second)
				atomic.AddInt32(&reqcount, 1)
				if nil != err {
					fmt.Printf("err:%s\n", err.Error())
				}
			}
		}()

		/*for j := 0; j < 40; j++ {
			go func() {
				for {
					arg := &testproto.Hello{Hello: proto.String("hello fasdfasdfasdfasdfjasjfjeiofjkaljfklasjfkljasdifjasijflkasdjl")}
					_, err := caller.SyncCall("hello", arg, time.Second)
					atomic.AddInt32(&reqcount, 1)
					if nil != err {
						fmt.Printf("err:%s\n", err.Error())
					}
				}
			}()
		}*/
	}
}

func testAsynCall(caller *test_rpc.Caller) {

	var callback1 func(interface{}, error)
	var callback2 func(interface{}, error)

	callback1 = func(msg interface{}, err error) {
		if nil != err {
			fmt.Printf("err:%s\n", err.Error())
		}
		arg := &testproto.Hello{Hello: proto.String("hello")}
		caller.AsynCall("hello", arg, time.Second, callback1)
	}

	callback2 = func(msg interface{}, err error) {
		if nil != err {
			fmt.Printf("err:%s\n", err.Error())
		}
		arg := &testproto.Hello{Hello: proto.String("hello fasdfasdfasdfasdfjasjfjeiofjkaljfklasjfkljasdifjasijflkasdjl")}
		caller.AsynCall("hello", arg, time.Second, callback2)
	}

	for j := 0; j < 10; j++ {
		arg := &testproto.Hello{Hello: proto.String("hello")}
		caller.AsynCall("hello", arg, time.Second, callback1)
	}

	for j := 0; j < 10; j++ {
		arg := &testproto.Hello{Hello: proto.String("hello fasdfasdfasdfasdfjasjfjeiofjkaljfklasjfkljasdifjasijflkasdjl")}
		caller.AsynCall("hello", arg, time.Second, callback2)
	}

}

func client(service string, count int) {

	for i := 0; i < count; i++ {
		go func() {
			caller := test_rpc.NewCaller()
			if err := caller.Dial(service, 10*time.Second); nil != err {
				fmt.Println(err.Error())
				return
			}

			//testSyncCall(caller)
			testAsynCall(caller)

		}()
	}
}

func main() {

	outLogger := golog.NewOutputLogger("log", "kendynet", 1024*1024*1000)
	kendynet.InitLogger(golog.New("rpc", outLogger))

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
		connectioncount, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Printf(err.Error())
			return
		}
		//让服务器先运行
		time.Sleep(time.Second)
		go client(service, connectioncount)

	}

	_, _ = <-sigStop

	return

}

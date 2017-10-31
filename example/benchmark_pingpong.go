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
	"runtime/pprof"
	"os/signal"
	"syscall"
)

func server(service string) {

	clientcount := int32(0)
	bytescount  := int32(0)
	packetcount := int32(0)



	go func() {
		for {
			time.Sleep(time.Second)
			tmp1 := atomic.LoadInt32(&bytescount)
			tmp2 := atomic.LoadInt32(&packetcount)
			atomic.StoreInt32(&bytescount,0)
			atomic.StoreInt32(&packetcount,0)
			fmt.Printf("clientcount:%d,transrfer:%d KB/s,packetcount:%d\n",clientcount,tmp1/1024,tmp2)			
		}
	}()

	server,err := tcp.NewListener("tcp4",service)
	if server != nil {
		fmt.Printf("server running on:%s\n",service)
		err = server.Start(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount,1)
			session.SetReceiver(codec.NewRawReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n",reason)
				atomic.AddInt32(&clientcount,-1)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					atomic.AddInt32(&bytescount,int32(len(event.Data.(kendynet.Message).Bytes())))
					atomic.AddInt32(&packetcount,int32(1))
					event.Session.SendMessage(event.Data.(kendynet.Message))
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
	
	client,err := tcp.NewConnector("tcp4",service)

	if err != nil {
		fmt.Printf("NewTcpClient failed:%s\n",err.Error())
		return
	}



	for i := 0; i < count ; i++ {
		session,_,err := client.Dial(time.Second * 10)
		if err != nil {
			fmt.Printf("Dial error:%s\n",err.Error())
		} else {
			session.SetReceiver(codec.NewRawReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n",reason)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					event.Session.SendMessage(event.Data.(kendynet.Message))
				}
			})
			session.Start()
			//send the first messge
			msg := kendynet.NewByteBuffer("hello")
			session.SendMessage(msg)
		}
	}
}


func main(){

	f, _ := os.Create("profile_file")
	pprof.StartCPUProfile(f)  // 开始cpu profile，结果写到文件f中
	defer pprof.StopCPUProfile()  // 结束profile


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

   	c := make(chan os.Signal) 
    signal.Notify(c, syscall.SIGINT)  //监听指定信号
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

    _ = <-c //阻塞直至有信号传入

	return

}



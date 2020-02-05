package main

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket/aio"
	connector "github.com/sniperHW/kendynet/socket/connector/aio"
	listener "github.com/sniperHW/kendynet/socket/listener/aio"
	"github.com/sniperHW/kendynet/timer"
	"os"
	"runtime"
	"sync/atomic"
	//"syscall"
	"github.com/sniperHW/kendynet/golog"
	"strconv"
	"time"
)

func server(service string) {

	clientcount := int32(0)
	bytescount := int32(0)
	packetcount := int32(0)

	timer.Repeat(time.Second, nil, func(_ *timer.Timer, ctx interface{}) {
		tmp1 := atomic.LoadInt32(&bytescount)
		tmp2 := atomic.LoadInt32(&packetcount)
		atomic.StoreInt32(&bytescount, 0)
		atomic.StoreInt32(&packetcount, 0)
		fmt.Printf("clientcount:%d,transrfer:%d KB/s,packetcount:%d\n", atomic.LoadInt32(&clientcount), tmp1/1024, tmp2)
	}, nil)

	server, err := listener.New("tcp4", service)
	if server != nil {
		fmt.Printf("server running on:%s\n", service)
		err = server.Serve(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount, 1)

			session.SetRecvTimeout(time.Second * 5)

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				atomic.AddInt32(&clientcount, -1)
				fmt.Println("client close:", reason, sess.GetUnderConn(), atomic.LoadInt32(&clientcount))
			})

			session.Start(func(msg *kendynet.Event) {
				if msg.EventType == kendynet.EventTypeError {
					msg.Session.Close(msg.Data.(error).Error(), 0)
				} else {
					var e error
					atomic.AddInt32(&bytescount, int32(len(msg.Data.(kendynet.Message).Bytes())))
					atomic.AddInt32(&packetcount, int32(1))
					for {
						e = msg.Session.SendMessage(msg.Data.(kendynet.Message))
						if e == nil {
							return
						} else if e != kendynet.ErrSendQueFull {
							break
						}
						runtime.Gosched()
					}
					if e != nil {
						fmt.Println("send error", e, msg.Session.GetUnderConn())
					}
				}
			})
		})

		if nil != err {
			fmt.Printf("TcpServer start failed %s\n", err)
		}

	} else {
		fmt.Printf("NewTcpServer failed %s\n", err)
	}
}

func client(service string, count int) {

	client, err := connector.New("tcp4", service)

	if err != nil {
		fmt.Printf("NewTcpClient failed:%s\n", err.Error())
		return
	}

	for i := 0; i < count; i++ {
		session, err := client.Dial(time.Second * 10)
		if err != nil {
			fmt.Printf("Dial error:%s\n", err.Error())
		} else {
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n", reason)
			})
			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					event.Session.SendMessage(event.Data.(kendynet.Message))
				}
			})
			//send the first messge
			msg := kendynet.NewByteBuffer("hello")
			session.SendMessage(msg)
			session.SendMessage(msg)
			session.SendMessage(msg)
		}
	}
}

func main() {

	outLogger := golog.NewOutputLogger("log", "kendynet", 1024*1024*1000)
	kendynet.InitLogger(golog.New("rpc", outLogger))
	kendynet.Debugln("start")

	aio.Init(1, runtime.NumCPU()*2, runtime.NumCPU()*2, nil)

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
		time.Sleep(10000000)
		go client(service, connectioncount)

	}

	_, _ = <-sigStop

	return

}

/*
func main() {

	aio.Init(1, runtime.NumCPU())

	go server("localhost:8110")

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT) //监听指定信号

	_ = <-c //阻塞直至有信号传入

	fmt.Println("exit")

	return

}
*/

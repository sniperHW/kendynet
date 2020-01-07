package main

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket/aio"
	"github.com/sniperHW/kendynet/timer"
	"net"
	"os"
	//"os/signal"
	"runtime"
	"sync/atomic"
	//"syscall"
	"strconv"
	"time"
)

func server(service string) {

	clientcount := int32(0)
	bytescount := int32(0)
	packetcount := int32(0)

	timer.Repeat(time.Second, nil, func(_ *timer.Timer) {
		tmp1 := atomic.LoadInt32(&bytescount)
		tmp2 := atomic.LoadInt32(&packetcount)
		atomic.StoreInt32(&bytescount, 0)
		atomic.StoreInt32(&packetcount, 0)
		fmt.Printf("clientcount:%d,transrfer:%d KB/s,packetcount:%d\n", atomic.LoadInt32(&clientcount), tmp1/1024, tmp2)
	})

	tcpAddr, err := net.ResolveTCPAddr("tcp", service)
	if err != nil {
		panic(err.Error())
	}

	ln, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		panic(err.Error())
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		atomic.AddInt32(&clientcount, 1)

		aioSocket := aio.NewAioSocket(conn)

		aioSocket.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			atomic.AddInt32(&clientcount, -1)
			fmt.Println("client close:", reason, sess.GetUnderConn(), atomic.LoadInt32(&clientcount))
		})

		aioSocket.Start(func(msg *kendynet.Event) {
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
	}
}

func dial(service string) (net.Conn, error) {
	dialer := &net.Dialer{}
	return dialer.Dial("tcp", service)
}

func client(service string, count int) {

	for i := 0; i < count; i++ {
		conn, err := dial(service)
		if err != nil {
			fmt.Printf("Dial error:%s\n", err.Error())
		} else {
			aioSocket := aio.NewAioSocket(conn)
			aioSocket.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n", reason)
			})
			aioSocket.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					event.Session.SendMessage(event.Data.(kendynet.Message))
				}
			})
			//send the first messge
			msg := kendynet.NewByteBuffer("hello")
			aioSocket.SendMessage(msg)
			aioSocket.SendMessage(msg)
			aioSocket.SendMessage(msg)
		}
	}
}

func main() {

	aio.Init(1, runtime.NumCPU()*2)

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

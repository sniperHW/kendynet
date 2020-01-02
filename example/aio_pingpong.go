package main

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	//"github.com/sniperHW/kendynet/golog"
	"github.com/sniperHW/kendynet/socket/aio"
	//connector "github.com/sniperHW/kendynet/socket/connector/tcp"
	//listener "github.com/sniperHW/kendynet/socket/listener/tcp"
	"github.com/sniperHW/kendynet/timer"
	"os"
	"os/signal"
	"runtime"
	//"runtime/pprof"
	//"strconv"
	"net"
	"sync/atomic"
	"syscall"
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

		w, q := aio.GetWatcherAndCompleteQueue()

		c, err := w.Watch(conn)
		if err != nil {
			fmt.Println(err)
			return
		}

		atomic.AddInt32(&clientcount, 1)

		aioSocket := aio.NewAioSocket(c, w, q)

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

func main() {

	aio.Init(1, 2)

	go server("localhost:8110")

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT) //监听指定信号

	_ = <-c //阻塞直至有信号传入

	fmt.Println("exit")

	return

}

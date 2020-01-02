package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/aio/codec"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/socket/aio"
	"github.com/sniperHW/kendynet/timer"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

func server(service string) {
	clientcount := int32(0)
	packetcount := int32(0)

	timer.Repeat(time.Second, nil, func(_ *timer.Timer) {
		tmp := atomic.LoadInt32(&packetcount)
		atomic.StoreInt32(&packetcount, 0)
		fmt.Printf("clientcount:%d,packetcount:%d\n", clientcount, tmp)
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

		w, rq, wq := aio.GetWatcherAndCompleteQueue()

		c, err := w.Watch(conn)
		if err != nil {
			fmt.Println(err)
			return
		}

		atomic.AddInt32(&clientcount, 1)

		aioSocket := aio.NewAioSocket(c, w, rq, wq)

		aioSocket.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			atomic.AddInt32(&clientcount, -1)
			fmt.Println("client close:", reason, sess.GetUnderConn(), atomic.LoadInt32(&clientcount))
		})

		aioSocket.SetEncoder(codec.NewPbEncoder(4096))
		aioSocket.SetReceiver(codec.NewPBReceiver(4096))

		aioSocket.Start(func(msg *kendynet.Event) {
			if msg.EventType == kendynet.EventTypeError {
				msg.Session.Close(msg.Data.(error).Error(), 0)
			} else {
				atomic.AddInt32(&packetcount, int32(1))
				msg.Session.Send(msg.Data.(proto.Message))
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

			w, rq, wq := aio.GetWatcherAndCompleteQueue()

			c, err := w.Watch(conn)
			if err != nil {
				fmt.Println(err)
				return
			}

			aioSocket := aio.NewAioSocket(c, w, rq, wq)

			aioSocket.SetEncoder(codec.NewPbEncoder(4096))
			aioSocket.SetReceiver(codec.NewPBReceiver(4096))
			aioSocket.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n", reason)
			})
			aioSocket.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					event.Session.Send(event.Data.(proto.Message))
				}
			})
			//send the first messge
			o := &testproto.Test{}
			o.A = proto.String("hello")
			o.B = proto.Int32(17)
			aioSocket.Send(o)
			aioSocket.Send(o)
			aioSocket.Send(o)
			aioSocket.Send(o)
			aioSocket.Send(o)
		}
	}
}

func main() {

	aio.Init(1, runtime.NumCPU())

	pb.Register(&testproto.Test{}, 1)
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

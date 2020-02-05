package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/aio/codec"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/socket/aio"
	connector "github.com/sniperHW/kendynet/socket/connector/aio"
	listener "github.com/sniperHW/kendynet/socket/listener/aio"
	"github.com/sniperHW/kendynet/timer"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

func server(service string) {

	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	clientcount := int32(0)
	packetcount := int32(0)

	timer.Repeat(time.Second, nil, func(_ *timer.Timer, ctx interface{}) {
		tmp := atomic.LoadInt32(&packetcount)
		atomic.StoreInt32(&packetcount, 0)
		fmt.Printf("clientcount:%d,packetcount:%d\n", clientcount, tmp)
	}, nil)

	server, err := listener.New("tcp4", service)
	if server != nil {
		fmt.Printf("server running on:%s\n", service)
		err = server.Serve(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount, 1)

			//session.SetRecvTimeout(time.Second * 5)

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				atomic.AddInt32(&clientcount, -1)
				fmt.Println("client close:", reason, sess.GetUnderConn(), atomic.LoadInt32(&clientcount))
			})

			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))

			session.Start(func(msg *kendynet.Event) {
				if msg.EventType == kendynet.EventTypeError {
					msg.Session.Close(msg.Data.(error).Error(), 0)
				} else {
					atomic.AddInt32(&packetcount, int32(1))
					msg.Session.Send(msg.Data.(proto.Message))
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
			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n", reason)
			})
			session.Start(func(event *kendynet.Event) {
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
			for i := 0; i < 50; i++ {
				session.Send(o)
			}
		}
	}

}

func main() {

	aio.Init(1, runtime.NumCPU()*2, runtime.NumCPU()*2, nil)

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

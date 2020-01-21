package main

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/message"
	connector "github.com/sniperHW/kendynet/socket/connector/websocket"
	listener "github.com/sniperHW/kendynet/socket/listener/websocket"
	"github.com/sniperHW/kendynet/timer"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
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
		fmt.Printf("clientcount:%d,transrfer:%d KB/s,packetcount:%d\n", clientcount, tmp1/1024, tmp2)
	}, nil)

	server, err := listener.New("tcp4", service, "/echo")
	if server != nil {
		fmt.Printf("server running on:%s\n", service)
		err = server.Serve(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount, 1)
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n", reason)
				atomic.AddInt32(&clientcount, -1)
			})
			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					//fmt.Printf("recv msg\n")
					atomic.AddInt32(&bytescount, int32(len(event.Data.(kendynet.Message).Bytes())))
					atomic.AddInt32(&packetcount, int32(1))
					event.Session.SendMessage(event.Data.(kendynet.Message))
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

	u := url.URL{Scheme: "ws", Host: service, Path: "/echo"}

	client, err := connector.New(u, nil)

	if err != nil {
		fmt.Printf("NewWSClient failed:%s\n", err.Error())
		return
	}

	for i := 0; i < count; i++ {
		session, _, err := client.Dial(10 * time.Second)
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
			err = session.SendMessage(message.NewWSMessage(message.WSTextMessage, "hello"))
			if err != nil {
				fmt.Printf("send err")
			}
		}
	}
}

func main() {

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

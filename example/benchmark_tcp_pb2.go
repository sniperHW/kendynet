package main

import (
	//"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/codec"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/golog"
	connector "github.com/sniperHW/kendynet/socket/connector/tcp"
	listener "github.com/sniperHW/kendynet/socket/listener/tcp"
	//"github.com/sniperHW/kendynet/timer"
	//"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

func check(str string, buff []byte) {
	l := len(buff)
	l = l / 8
	for i := 0; i < l; i++ {
		var j int
		for j = 0; j < 8; j++ {
			if buff[i*8+j] != 0 {
				break
			}
		}
		if j == 8 {
			kendynet.Infoln(str, buff)
			panic("check error")
			return
		}
	}
}

func server(service string) {
	l, err := listener.New("tcp", service)
	if nil == err {
		fmt.Printf("server running\n")
		l.Serve(func(session kendynet.StreamSession) {
			go func() {
				conn := session.GetUnderConn().(net.Conn)
				recvBuff := make([]byte, 4096)
				for {
					n, err := conn.Read(recvBuff)
					if 0 == n || nil != err {
						fmt.Println("read error", err)
						return
					}

					//serverRecvBytes += int64(n)

					sendBuff := recvBuff[:n]

					check("server", sendBuff)

					for {
						n, err := conn.Write(sendBuff)
						if nil != err {
							fmt.Println("write error", err)
							return
						} else {
							//serverSendBytes += int64(n)
							if n == len(sendBuff) {
								break
							} else {
								sendBuff = sendBuff[n:]
							}
						}
					}
				}
			}()
		})
	}
}

func client(service string, count int) {

	packetcount := int32(0)

	go func() {
		for {
			time.Sleep(time.Second)
			tmp := atomic.LoadInt32(&packetcount)
			atomic.StoreInt32(&packetcount, 0)
			fmt.Printf("packetcount:%d\n", tmp)
		}
	}()

	client, err := connector.New("tcp4", service)

	if err != nil {
		fmt.Printf("NewTcpClient failed:%s\n", err.Error())
		return
	}

	for i := 0; i < count; i++ {
		session, err := client.Dial(10 * time.Second)
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
					//fmt.Printf("client on msg\n")
					atomic.AddInt32(&packetcount, int32(1))
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

	outLogger := golog.NewOutputLogger("log", "kendynet", 1024*1024*1000)
	kendynet.InitLogger(golog.New("rpc", outLogger))
	kendynet.Debugln("start")

	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

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

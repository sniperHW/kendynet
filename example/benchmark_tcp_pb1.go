package main

import (
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/codec"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/golog"
	connector "github.com/sniperHW/kendynet/socket/connector/tcp"
	listener "github.com/sniperHW/kendynet/socket/listener/tcp"
	"github.com/sniperHW/kendynet/timer"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

func server(service string) {
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
			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Printf("server client close:%s\n", reason)
				atomic.AddInt32(&clientcount, -1)
			})
			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					//fmt.Printf("server on msg\n")
					atomic.AddInt32(&packetcount, int32(1))
					event.Session.Send(event.Data.(proto.Message))
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

func client(service string, count int) {
	client, err := connector.New("tcp4", service)

	if err != nil {
		fmt.Printf("NewTcpClient failed:%s\n", err.Error())
		return
	}

	session, err := client.Dial(time.Second * 10)
	if err != nil {
		fmt.Printf("Dial error:%s\n", err.Error())
	} else {

		conn := session.GetUnderConn().(net.Conn)
		encoder := codec.NewPbEncoder(4096)
		o := &testproto.Test{}
		o.A = proto.String("hello")
		o.B = proto.Int32(17)
		msg, _ := encoder.EnCode(o)

		recvBuff := make([]byte, 65536)

		var buff bytes.Buffer

		for {

			buff.Reset()

			c := rand.Int()%25 + 25

			for i := 0; i < c; i++ {
				buff.Write(msg.Bytes())
			}

			b := buff.Bytes()

			for {
				n, err := conn.Write(b)
				if nil != err {
					fmt.Println("write error", err)
					return
				} else {
					if n == len(b) {
						break
					} else {
						b = b[n:]
					}
				}
			}

			/*recvBuff := make([]byte, len(buff.Bytes()))

			n, err := io.ReadFull(conn, recvBuff)
			if 0 == n || nil != err {
				fmt.Println("read error", err)
				return
			}

			clientRecvBytes += int64(n)
			clientRecv = clientRecvBytes / int64(len(msg.Bytes()))
			check(recvBuff)*/

			recvBytes := 0
			for recvBytes < len(buff.Bytes()) {
				n, err := conn.Read(recvBuff)
				if 0 == n || nil != err {
					fmt.Println("read error", err)
					return
				}
				//clientRecvBytes += int64(n)
				//clientRecv = clientRecvBytes / int64(len(msg.Bytes()))
				check("client", recvBuff[:n])
				recvBytes += n
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

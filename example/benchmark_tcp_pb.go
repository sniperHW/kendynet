package main

import(
	"time"
	"sync/atomic"
	"strconv"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/timer"
	"github.com/sniperHW/kendynet/socket/stream_socket/tcp"
	codec "github.com/sniperHW/kendynet/example/codec/stream_socket"		
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/golog"
)

func server(service string) {
	clientcount := int32(0)
	packetcount := int32(0)

	timer.Repeat(time.Second,nil,func (_ timer.TimerID) {
		tmp := atomic.LoadInt32(&packetcount)
		atomic.StoreInt32(&packetcount,0)
		fmt.Printf("clientcount:%d,packetcount:%d\n",clientcount,tmp)	
	})

	server,err := tcp.NewListener("tcp4",service)
	if server != nil {
		fmt.Printf("server running on:%s\n",service)
		err = server.Start(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount,1)
			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("server client close:%s\n",reason)
				atomic.AddInt32(&clientcount,-1)
			})
			session.Start(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					//fmt.Printf("server on msg\n")
					atomic.AddInt32(&packetcount,int32(1))
					event.Session.Send(event.Data.(proto.Message))
				}
			})
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
		session,err := client.Dial(10 * time.Second)
		if err != nil {
			fmt.Printf("Dial error:%s\n",err.Error())
		} else {
			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n",reason)
			})
			session.Start(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					//fmt.Printf("client on msg\n")
					event.Session.Send(event.Data.(proto.Message))
				}
			})
			//send the first messge
			o := &testproto.Test{}
			o.A = proto.String("hello")
			o.B = proto.Int32(17)
			session.Send(o)
			session.Send(o)
			session.Send(o)
			session.Send(o)
			session.Send(o)
		}
	}
}


func main(){

	outLogger := golog.NewOutputLogger("log","kendynet",1024*1024*1000)
	kendynet.InitLogger(outLogger)

	pb.Register(&testproto.Test{},1)
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
		connectioncount,err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Printf(err.Error())
			return
		}
		//让服务器先运行
		time.Sleep(10000000)
		go client(service,connectioncount)

	}

	_,_ = <- sigStop

	return

}



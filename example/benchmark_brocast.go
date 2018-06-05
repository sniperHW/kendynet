package main

import(
	"time"
	"sync/atomic"
	"strconv"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket/stream_socket/tcp"
	codec "github.com/sniperHW/kendynet/example/codec/stream_socket"		
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/example/pb"
)

const (
	ev_newclient  = 1
	ev_disconnect = 2
	ev_error      = 3
	ev_message    = 4
)

type event struct {
	eventType byte
	session   kendynet.StreamSession
	data      interface{}
}


/*
*  使用event_queue把多线程事件转换为单线程处理 
*/

func server(service string) {
	clientcount := int32(0)
	packetcount := int32(0)

	//clientMap只被单个goroutine访问，不需要任何保护
	clientMap := make(map[kendynet.StreamSession]bool)

	go func() {
		for {
			time.Sleep(time.Second)
			tmp := atomic.LoadInt32(&packetcount)
			atomic.StoreInt32(&packetcount,0)
			fmt.Printf("clientcount:%d,packetcount:%d\n",clientcount,tmp)			
		}
	}()

	evQueue := kendynet.NewEventQueue()

	server,err := tcp.NewListener("tcp4",service)
	if server != nil {
		go func() {
			fmt.Printf("server running on:%s\n",service)
			err = server.Start(func(session kendynet.StreamSession) {
				session.SetEncoder(codec.NewPbEncoder(4096))
				session.SetReceiver(codec.NewPBReceiver(4096))
				session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
					evQueue.PostEvent(&event{eventType:ev_error,session:sess,data:reason})
				})
				session.Start(func (ev *kendynet.Event) {
					if ev.EventType == kendynet.EventTypeError {
						evQueue.PostEvent(&event{eventType:ev_disconnect,session:session,data:ev.Data})
					} else {
						evQueue.PostEvent(&event{eventType:ev_message,session:session,data:ev.Data})
					}
				})
				evQueue.PostEvent(&event{eventType:ev_newclient,session:session})
			})

			if nil != err {
				fmt.Printf("TcpServer start failed %s\n",err)			
			}
		}()

		evQueue.Start(func (ev interface{}) {
			_ev := ev.(*event)
			if _ev.eventType == ev_newclient {
				atomic.AddInt32(&clientcount,1)
				clientMap[_ev.session] = true
			} else if _ev.eventType == ev_disconnect {
				atomic.AddInt32(&clientcount,-1)
				delete(clientMap,_ev.session)
			} else if _ev.eventType == ev_error {
				_ev.session.Close(_ev.data.(error).Error(),0)
			} else {
				for s,_ := range clientMap {
					s.Send(_ev.data.(proto.Message))
				}
				atomic.AddInt32(&packetcount,int32(len(clientMap)))
			}
		})

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
			selfID := i + 1
			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(codec.NewPBReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n",reason)
			})
			session.Start(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					msg := event.Data.(*testproto.BrocastPingpong)
					if msg.GetId() == int64(selfID) {
						event.Session.Send(event.Data.(proto.Message))
					}
				}
			})
			//send the first messge
			o := &testproto.BrocastPingpong{}
			o.Id = proto.Int64(int64(selfID))
			o.Message = proto.String("hello")
			session.Send(o)
		}
	}
}


func main(){
	pb.Register(&testproto.BrocastPingpong{},1)
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



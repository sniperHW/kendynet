package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/codec"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/socket/aio"
	connector "github.com/sniperHW/kendynet/socket/connector/aio"
	listener "github.com/sniperHW/kendynet/socket/listener/aio"
	"github.com/sniperHW/kendynet/timer"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type BufferPool struct {
	pool sync.Pool
}

const PoolBuffSize uint64 = 65 * 1024

func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, PoolBuffSize)
			},
		},
	}
}

func (p *BufferPool) Acquire() []byte {
	return p.pool.Get().([]byte)
}

func (p *BufferPool) Release(buff []byte) {
	if uint64(cap(buff)) == PoolBuffSize {
		p.pool.Put(buff[:cap(buff)])
	}
}

var bufferpool *BufferPool = NewBufferPool()
var makeBuffCount int32 = 0

type InBoundProcessor struct {
	buffer []byte
	w      int
	r      int
	name   string
}

//供aio socket使用
func (this *InBoundProcessor) GetRecvBuff() []byte {
	if len(this.buffer) == 0 {
		//sharebuffer模式
		return nil
	} else {
		//之前有包没接收完，先把这个包接收掉
		return this.buffer[this.w:]
	}
}

func (this *InBoundProcessor) OnData(data []byte) {
	if len(this.buffer) == 0 {
		this.buffer = data
	}
	this.w += len(data)
}

func (this *InBoundProcessor) Unpack() (interface{}, error) {
	if this.r == this.w {
		return nil, nil
	} else {
		msg, packetSize, err := pb.Decode(this.buffer, this.r, this.w, 4096)
		if nil != msg {
			this.r += packetSize
			if this.r == this.w {
				this.r = 0
				this.w = 0
				bufferpool.Release(this.buffer)
				this.buffer = nil
			}
		} else if nil == err {
			atomic.AddInt32(&makeBuffCount, 1)
			//新开一个buffer把未接完整的包先接收掉,处理完这个包之后再次启用sharebuffer模式
			buff := make([]byte, packetSize)
			copy(buff, this.buffer[this.r:this.w])
			this.w = this.w - this.r
			this.r = 0
			bufferpool.Release(this.buffer)
			this.buffer = buff
		}
		return msg, err
	}
}

func (this *InBoundProcessor) OnSocketClose() {
	bufferpool.Release(this.buffer)
}

//供阻塞式socket使用
func (this *InBoundProcessor) ReceiveAndUnpack(_ kendynet.StreamSession) (interface{}, error) {
	return nil, nil
}

var aioService *aio.SocketService

func server(service string) {

	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	clientcount := int32(0)
	packetcount := int32(0)

	timer.Repeat(time.Second, func(_ *timer.Timer, ctx interface{}) {
		tmp := atomic.LoadInt32(&packetcount)
		atomic.StoreInt32(&packetcount, 0)
		tmp2 := atomic.LoadInt32(&makeBuffCount)
		atomic.StoreInt32(&makeBuffCount, 0)
		fmt.Printf("clientcount:%d,packetcount:%d,makeBuffCount:%d\n", clientcount, tmp, tmp2)
	}, nil)

	server, err := listener.New(aioService, "tcp4", service)
	if server != nil {
		fmt.Printf("server running on:%s\n", service)
		err = server.Serve(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount, 1)

			//session.SetRecvTimeout(time.Second * 5)

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				atomic.AddInt32(&clientcount, -1)
				fmt.Println("client close:", reason, sess.GetUnderConn(), atomic.LoadInt32(&clientcount))
			})

			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetInBoundProcessor(&InBoundProcessor{name: "server"})

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				atomic.AddInt32(&packetcount, int32(1))
				s.Send(msg.(proto.Message))
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

	client, err := connector.New(aioService, "tcp4", service)

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
			session.SetInBoundProcessor(&InBoundProcessor{name: "client"})
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				fmt.Printf("client client close:%s\n", reason)
			})

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				s.Send(msg.(proto.Message))
			})

			//send the first messge
			o := &testproto.Test{}
			o.A = proto.String(strings.Repeat("a", 512))
			o.B = proto.Int32(17)
			for i := 0; i < 50; i++ {
				session.Send(o)
			}
		}
	}

}

func main() {

	aioService = aio.NewSocketService(bufferpool)

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

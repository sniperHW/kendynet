package main

import(
	"time"
	"sync/atomic"
	"strconv"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/stream_socket/tcp"
	codec "github.com/sniperHW/kendynet/codec/stream_socket"		
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/pb"
	"compress/gzip"
	"github.com/sniperHW/kendynet/stream_socket"
	"bytes"
	"runtime/pprof"
	"os/signal"
	"syscall"
	"io/ioutil"
)

type ZipPBReceiver struct {
	recvBuff       [] byte
	PBBuffers      bytes.Buffer
	pbHead         int
	tail           int
	r             *gzip.Reader
}

func (this *ZipPBReceiver) unPack() (interface{},error) {
	if this.PBBuffers.Len() - this.pbHead > 0 {
		pbBuff := this.PBBuffers.Bytes()
		unpackSize := this.PBBuffers.Len() - this.pbHead
		msg,dataLen,err := pb.Decode(pbBuff,uint64(this.pbHead),uint64(this.pbHead + unpackSize),4096)
		if err != nil {
			fmt.Printf("err:%s\n",err.Error())
			return nil,err
		}
		this.pbHead += int(dataLen)
		if this.pbHead >= this.PBBuffers.Len() {
			this.pbHead = 0
			this.PBBuffers.Reset()
		}
		return msg,err
	} else {
		ok,err := this.unZip()
		if ok {
			return this.unPack()
		} else {
			return nil,err
		}
	}
}

func (this *ZipPBReceiver) unZip() (bool,error) {

	if this.tail == 0 {
		return false,nil
	}

	reader := kendynet.NewByteBuffer(this.recvBuff,this.tail)
	payload,err := reader.GetUint32(0)
	if err != nil {
		return false,err
	}

	zipBytes,err := reader.GetBytes(4,uint64(payload))
	if err != nil {
		return false,err
	}
	b := bytes.NewBuffer(zipBytes)

	if nil == this.r {
		this.r,err = gzip.NewReader(b)
	} else {
		err = this.r.Reset(b)
	}

	if err != nil {
		return false,err
	}

	data,err := ioutil.ReadAll(this.r)

	this.PBBuffers.Write(data)

	if this.tail < len(this.recvBuff) {
		copy(this.recvBuff,this.recvBuff[4 + payload:])
	}

	this.tail -= int(4 + payload)

	return true,nil
}

func (this *ZipPBReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{},error) {

	var msg interface{}
	var err error
	for {
		msg,err = this.unPack()
		if nil != msg {
			break
		}
		if err == nil {	
			n,err := sess.(*stream_socket.StreamSocket).Read(this.recvBuff[this.tail:])
			if err != nil {
				return nil,err
			}
			this.tail += n
		} else {
			break
		}
	}
	return msg,err	
}

func NewZipPBReceiver(buffsize uint64)(*ZipPBReceiver){
	return &ZipPBReceiver{recvBuff:make([]byte,buffsize)}
}

type ZipBuffProcessor struct {
	b          bytes.Buffer
	w		  *gzip.Writer     
}

func (this *ZipBuffProcessor) Process(packets []kendynet.Message) []kendynet.Message {
	//将输入的所有包合并成一个包进行zip压缩
	var ret  []kendynet.Message
	for i := 0; i < len(packets); i++ {
		msg := packets[i].(kendynet.Message)
		data := msg.Bytes()
		this.w.Write(data)
	}
	this.w.Flush()
	byteBuffer := kendynet.NewByteBuffer(this.b.Len() + 4)
	byteBuffer.PutUint32(0,uint32(len(this.b.Bytes())))
	byteBuffer.PutBytes(4,this.b.Bytes())
	this.b.Reset()
	this.w.Reset(&this.b)
	ret = append(ret,byteBuffer)
	return ret
}

func NewZipBuffProcessor()(*ZipBuffProcessor){
	z := &ZipBuffProcessor{}
	z.w = gzip.NewWriter(&z.b)
	return z
}

func server(service string) {
	clientcount := int32(0)
	packetcount := int32(0)

	go func() {
		for {
			time.Sleep(time.Second)
			tmp := atomic.LoadInt32(&packetcount)
			atomic.StoreInt32(&packetcount,0)
			fmt.Printf("clientcount:%d,packetcount:%d\n",clientcount,tmp)			
		}
	}()

	server,err := tcp.NewListener("tcp4",service)
	if server != nil {
		fmt.Printf("server running on:%s\n",service)
		err = server.Start(func(session kendynet.StreamSession) {
			atomic.AddInt32(&clientcount,1)
			session.SetEncoder(codec.NewPbEncoder(4096))
			session.SetReceiver(NewZipPBReceiver(4096))
			session.(*stream_socket.StreamSocket).SetSendBuffProcessor(NewZipBuffProcessor())
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("server client close:%s\n",reason)
				atomic.AddInt32(&clientcount,-1)
			})
			j := 0
			session.Start(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					j++
					atomic.AddInt32(&packetcount,int32(1))
					//event.Session.Send(event.Data.(proto.Message))
					event.Session.PostSend(event.Data.(proto.Message))
					if j % 10 == 0 {
						event.Session.Flush()
					}
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
			session.SetReceiver(NewZipPBReceiver(4096))
			session.(*stream_socket.StreamSocket).SetSendBuffProcessor(NewZipBuffProcessor())
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client client close:%s\n",reason)
			})
			j := 0
			session.Start(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					j++
					event.Session.PostSend(event.Data.(proto.Message))
					if j % 10 == 0 {
						event.Session.Flush()
					}
				}
			})
			//使用postSend,10个包合并成一个由NewZipBuffProcessor执行压缩后发送
			for j := 0; j < 10; j++ {
				o := &testproto.Test{}
				o.A = proto.String("hello")
				o.B = proto.Int32(17)
				session.PostSend(o)
			}
			session.Flush()
		}
	}
}


func main(){

	f, _ := os.Create("profile_file")
	pprof.StartCPUProfile(f)  // 开始cpu profile，结果写到文件f中
	defer pprof.StopCPUProfile()  // 结束profile

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

   	c := make(chan os.Signal) 
    signal.Notify(c, syscall.SIGINT)  //监听指定信号

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

	_ = <-c //阻塞直至有信号传入

	fmt.Printf("server over\n")

	return

}



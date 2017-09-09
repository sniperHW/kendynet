package main

import(
	"net"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/protocal/protocal_stream_socket"		
)

func main(){

	if len(os.Args) < 2 {
		fmt.Printf("usage ./testserver ip:port\n")
		return
	}

	service := os.Args[1]
	tcpAddr,err := net.ResolveTCPAddr("tcp4", service)
	if err != nil{
		fmt.Printf("ResolveTCPAddr error\n")
		return
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil{
		fmt.Printf("ListenTCP error\n")
	}

	fmt.Printf("server running on %s\n",service)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		session := kendynet.NewStreamSocket(conn)
		session.SetReceiver(protocal_stream_socket.NewBinaryReceiver(4096))
		session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
			fmt.Printf("client close:%s\n",reason)
		})
		session.SetPacketCallBack(func (sess kendynet.StreamSession,msg interface{},err error) {
			if nil != err {
				session.Close("none",0)
			} else {
				session.SendBuff(msg.(*kendynet.ByteBuffer))
			}
		})
		session.Start()
	}
}
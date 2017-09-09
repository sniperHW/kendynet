package main

import(
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
	server,err := kendynet.NewTcpServer("tcp4",service)
	if server != nil {
		fmt.Printf("server running on:%s\n",service)
		err = server.Start(func(session kendynet.StreamSession) {
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
		})

		if nil != err {
			fmt.Printf("TcpServer start failed %s\n",err)			
		}

	} else {
		fmt.Printf("NewTcpServer failed %s\n",err)
	}
}
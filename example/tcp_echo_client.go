package main

import(
	"fmt"
	"os"
	"strconv"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/protocal/protocal_stream_socket"		
)

func main(){

	if len(os.Args) < 3 {
		fmt.Printf("usage ./tcp_echo_client ip:port count\n")
		return
	}
	service := os.Args[1]
	count,err := strconv.Atoi(os.Args[2])

	if err != nil {
		fmt.Printf("invaild agr2:%s\n",err.Error())		
		return
	}

	client,err := kendynet.NewTcpClient("tcp4",service)

	if err != nil {
		fmt.Printf("NewTcpClient failed:%s\n",err.Error())
		return
	}



	for i := 0; i < count ; i++ {
		session,_,err := client.Dial()
		if err != nil {
			fmt.Printf("Dial error:%s\n",err.Error())
		} else {
			session.SetReceiver(protocal_stream_socket.NewBinaryReceiver(4096))
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n",reason)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					session = nil
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					fmt.Printf("%s\n",(string)(event.Data.(kendynet.Message).Bytes()))
					event.Session.SendMessage(event.Data.(kendynet.Message))
				}
			})
			session.Start()
			//send the first messge
			msg := kendynet.NewByteBuffer("hello")
			session.SendMessage(msg)
		}
	}

	sigStop := make(chan bool)
	_,_ = <- sigStop
}
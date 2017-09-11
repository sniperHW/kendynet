package main

import(
	"fmt"
	"os"
	"strconv"
	"net/url"
	"github.com/sniperHW/kendynet"
	"github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet/protocal/protocal_websocket"		
)

func main(){

	if len(os.Args) < 3 {
		fmt.Printf("usage ./ws_echo_client ip:port count\n")
		return
	}
	service := os.Args[1]
	count,err := strconv.Atoi(os.Args[2])

	if err != nil {
		fmt.Printf("invaild agr2:%s\n",err.Error())		
		return
	}

	u := url.URL{Scheme: "ws", Host: service, Path: "/echo"}

	client,err := kendynet.NewWSClient(u,nil,websocket.DefaultDialer)

	if err != nil {
		fmt.Printf("NewWSClient failed:%s\n",err.Error())
		return
	}

	for i := 0; i < count ; i++ {
		session,_,err := client.Dial()
		if err != nil {
			fmt.Printf("Dial error:%s\n",err.Error())
		} else {
			session.SetReceiver(protocal_websocket.NewRawReceiver())
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n",reason)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					session = nil
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					fmt.Printf("%s\n",(string)(event.Data.(kendynet.Message).Bytes()))
					err := event.Session.SendMessage(event.Data.(kendynet.Message))
					if err != nil {
						fmt.Printf("SendMessage error:%s",err.Error())
						event.Session.Close(err.Error(),0)
					}
				}
			})
			session.Start()
			//send the first messge
			msg := kendynet.NewWSMessage(kendynet.WSTextMessage , "hello")
			err := session.SendMessage(msg)
			if err != nil {
				fmt.Printf("SendMessage error:%s",err.Error())
				session.Close(err.Error(),0)
			}
		}
	}

	sigStop := make(chan bool)
	_,_ = <- sigStop
}
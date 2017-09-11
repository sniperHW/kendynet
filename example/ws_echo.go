package main

import(
	"fmt"
	"net/http"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/websocket"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet/protocal/protocal_websocket"		
)

func main(){

	upgrader := &gorilla.Upgrader{}

	upgrader.CheckOrigin = func(r *http.Request) bool {
		// allow all connections by default
		return true
	}
	server,err := websocket.NewServer("tcp4","127.0.0.1:8010","/echo",upgrader)
	if server != nil {
		fmt.Printf("server running\n")
		err = server.Start(func(session kendynet.StreamSession) {
			fmt.Printf("new client\n")
			session.SetReceiver(protocal_websocket.NewRawReceiver())
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n",reason)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					fmt.Printf("recvmsg\n")
					err := event.Session.SendMessage(event.Data.(kendynet.Message))
					if err != nil {
						fmt.Printf("SendMessage error:%s",err.Error())
						event.Session.Close(err.Error(),0)
					}
				}
			})
			session.Start()
		})

		if nil != err {
			fmt.Printf("Server start failed %s\n",err)			
		}

	} else {
		fmt.Printf("NewWSServer failed %s\n",err)
	}
}
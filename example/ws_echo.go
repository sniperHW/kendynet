package main

import(
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet/protocal/protocal_websocket"		
)

func main(){

	server,err := kendynet.NewWSServer("tcp4","localhost:8010","/echo",&websocket.Upgrader{})
	if server != nil {
		fmt.Printf("server running\n")
		err = server.Start(func(session kendynet.StreamSession) {
			session.SetReceiver(protocal_websocket.NewRawReceiver())
			session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
				fmt.Printf("client close:%s\n",reason)
			})
			session.SetEventCallBack(func (event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(),0)
				} else {
					event.Session.SendMessage(event.Data.(kendynet.Message))
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
package protocal_websocket

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/websocket"	
)

type WSSocketRawReceiver struct {

}

func (this *WSSocketRawReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{},error) {
	mt, message, err := sess.(*websocket.WebSocket).ReadMessage()
	if err != nil {
		return nil,err
	} else {
		//fmt.Printf("%d,%s\n",mt,(string)(message))
		return websocket.NewMessage(mt,message),nil
	}
}


func NewRawReceiver()(*WSSocketRawReceiver){
	return &WSSocketRawReceiver{}
}

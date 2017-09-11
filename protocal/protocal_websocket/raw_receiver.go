package protocal_websocket

import (
	"github.com/sniperHW/kendynet"
	"github.com/gorilla/websocket"
	//"fmt"
)

type WSSocketRawReceiver struct {

}

func (this *WSSocketRawReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{},error) {
	mt, message, err := sess.GetUnderConn().(*websocket.Conn).ReadMessage()
	if err != nil {
		return nil,err
	} else {
		//fmt.Printf("%d,%s\n",mt,(string)(message))
		return kendynet.NewWSMessage(mt,message),nil
	}
}


func NewRawReceiver()(*WSSocketRawReceiver){
	return &WSSocketRawReceiver{}
}

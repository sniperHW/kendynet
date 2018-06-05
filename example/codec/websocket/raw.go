package websocket

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket/websocket"	
)

/*
*   无封包结构，直接将收到的所有数据返回
*/

type RawReceiver struct {

}

func (this *RawReceiver) ReceiveAndUnpack(sess kendynet.StreamSession) (interface{},error) {
	mt, message, err := sess.(*websocket.WebSocket).Read()
	if err != nil {
		return nil,err
	} else {
		return websocket.NewMessage(mt,message),nil
	}
}


func NewRawReceiver()(*RawReceiver){
	return &RawReceiver{}
}

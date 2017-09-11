package kendynet

import (
    "sync/atomic"
	"net"
	"net/http"
	"github.com/gorilla/websocket"
	"fmt"
)

type WSServer struct{
    listener    *net.TCPListener
    upgrader    *websocket.Upgrader
    origin       string
    started      int32
}

func NewWSServer(nettype string,service string,origin string,upgrader *websocket.Upgrader) (*WSServer,error) {

	if upgrader == nil {
		return nil,ErrWSInvaildUpgrader
	}

	tcpAddr,err := net.ResolveTCPAddr(nettype, service)
	if err != nil{
		return nil,err
	}
	listener, err := net.ListenTCP(nettype, tcpAddr)
	if err != nil{
		return nil,err
	}
	server := &WSServer{listener:listener,origin:origin,upgrader:upgrader}
	return server,nil
}

func (this *WSServer) Close() {
	if nil != this.listener {
		this.listener.Close()
	}
}

func (this *WSServer) Start(onNewClient func(StreamSession)) error {

    if nil == onNewClient {
        return ErrInvaildNewClientCB
    }

    if !atomic.CompareAndSwapInt32(&this.started,0,1) {
        return ErrServerStarted
    }

    http.HandleFunc(this.origin, func (w http.ResponseWriter, r *http.Request) {
		c, err := this.upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Printf("Upgrade failed:%s\n",err.Error())
			return
		}
		sess := NewWSSocket(c)
		onNewClient(sess)
	})

	err := http.Serve(this.listener,nil)
	if err != nil {
		fmt.Printf("%s\n",err.Error())
	}

	this.listener.Close()

	return err
}


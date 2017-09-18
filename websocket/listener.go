package websocket

import (
    "sync/atomic"
	"net"
	"net/http"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"fmt"
)

type Listener struct{
    listener    *net.TCPListener
    upgrader    *gorilla.Upgrader
    origin       string
    started      int32
}

func NewListener(nettype string,service string,origin string,upgrader *gorilla.Upgrader) (*Listener,error) {

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
	return &Listener{listener:listener,origin:origin,upgrader:upgrader},nil
}

func (this *Listener) Close() {
	if nil != this.listener {
		this.listener.Close()
	}
}

func (this *Listener) Start(onNewClient func(kendynet.StreamSession)) error {

    if nil == onNewClient {
        return kendynet.ErrInvaildNewClientCB
    }

    if !atomic.CompareAndSwapInt32(&this.started,0,1) {
        return kendynet.ErrServerStarted
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


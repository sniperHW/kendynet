package websocket

import (
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket"
	"net"
	"net/http"
	"sync/atomic"
)

type Listener struct {
	listener *net.TCPListener
	upgrader *gorilla.Upgrader
	origin   string
	started  int32
}

func New(nettype string, service string, origin string, upgrader ...*gorilla.Upgrader) (*Listener, error) {
	tcpAddr, err := net.ResolveTCPAddr(nettype, service)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP(nettype, tcpAddr)
	if err != nil {
		return nil, err
	}

	l := &Listener{
		listener: listener,
		origin:   origin,
	}

	if len(upgrader) > 0 {
		l.upgrader = upgrader[0]
	} else {
		l.upgrader = &gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// allow all connections by default
				return true
			},
		}
	}

	return l, nil
}

func (this *Listener) Close() {
	if nil != this.listener {
		this.listener.Close()
	}
}

func (this *Listener) Serve(onNewClient func(kendynet.StreamSession)) error {

	if nil == onNewClient {
		return kendynet.ErrInvaildNewClientCB
	}

	if !atomic.CompareAndSwapInt32(&this.started, 0, 1) {
		return kendynet.ErrServerStarted
	}

	http.HandleFunc(this.origin, func(w http.ResponseWriter, r *http.Request) {
		c, err := this.upgrader.Upgrade(w, r, nil)
		if err != nil {
			kendynet.Errorf("wssocket Upgrade failed:%s\n", err.Error())
			return
		}
		sess := socket.NewWSSocket(c)
		onNewClient(sess)
	})

	err := http.Serve(this.listener, nil)
	if err != nil {
		kendynet.Errorf("http.Serve() failed:%s\n", err.Error())
	}

	this.listener.Close()

	return err
}

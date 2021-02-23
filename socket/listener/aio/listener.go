package aio

import (
    "github.com/sniperHW/kendynet"
    "github.com/sniperHW/kendynet/socket/aio"
    "net"
    "sync/atomic"
)

type Listener struct {
    listener *net.TCPListener
    started  int32
    closed   int32
    s        *aio.SocketSerice
}

func New(s *aio.SocketSerice, nettype, service string) (*Listener, error) {
    tcpAddr, err := net.ResolveTCPAddr(nettype, service)
    if err != nil {
        return nil, err
    }
    listener, err := net.ListenTCP(nettype, tcpAddr)
    if err != nil {
        kendynet.GetLogger().Errorf("ListenTCP service:%s error:%s\n", service, err.Error())
        return nil, err
    }
    return &Listener{listener: listener, s: s}, nil
}

func (this *Listener) Close() {
    if atomic.CompareAndSwapInt32(&this.closed, 0, 1) {
        if nil != this.listener {
            this.listener.Close()
        }
    }
}

func (this *Listener) Serve(onNewClient func(kendynet.StreamSession)) error {

    if nil == onNewClient {
        return kendynet.ErrInvaildNewClientCB
    }

    if !atomic.CompareAndSwapInt32(&this.started, 0, 1) {
        return kendynet.ErrServerStarted
    }

    for {
        conn, err := this.listener.Accept()
        if err != nil {
            if atomic.LoadInt32(&this.closed) == 1 {
                return nil
            }

            if ne, ok := err.(net.Error); ok && ne.Temporary() {
                kendynet.GetLogger().Errorf("accept temp err: %v", ne)
                continue
            } else {
                return err
            }

        } else {

            onNewClient(aio.NewSocket(this.s, conn))
        }
    }
}

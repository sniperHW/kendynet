package kendynet

import (
    "net"
    "sync/atomic"
)

type TcpServer struct{
    listener    *net.TCPListener
    started      int32
}

func NewTcpServer(nettype,service string) (*TcpServer,error) {
    tcpAddr,err := net.ResolveTCPAddr(nettype, service)
    if err != nil{
        return nil,err
    }
    listener, err := net.ListenTCP(nettype, tcpAddr)
    if err != nil{
        return nil,err
    }
    tcpServer := &TcpServer{listener:listener}
    
    return tcpServer,nil
}

func (this *TcpServer) Close() {
    if nil != this.listener {
        this.listener.Close()
    }
}


func (this *TcpServer) Start(onNewClient func(StreamSession)) error {

    if nil == onNewClient {
        return ErrInvaildNewClientCB
    }

    if !atomic.CompareAndSwapInt32(&this.started,0,1) {
        return ErrServerStarted
    }

    for {
        conn, err := this.listener.Accept()
        if err != nil {
            this.listener.Close()
            return err
        }
        onNewClient(NewStreamSocket(conn))
    }
}
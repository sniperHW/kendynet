package tcp

import (
    "net"
    "github.com/sniperHW/kendynet"
)

type TcpClient struct{
	nettype string
	addr *net.TCPAddr
}

func NewClient(nettype string,service string) (*TcpClient,error) {
	addr,err := net.ResolveTCPAddr(nettype, service)
	if nil != err {
		return nil,err
	}
	tcpClient := &TcpClient{nettype:nettype,addr:addr}
	return tcpClient,nil
}

func (this *TcpClient) Dial() (kendynet.StreamSession,interface{},error) {
	conn, err := net.DialTCP(this.nettype, nil, this.addr)
	if err != nil {
		return nil,nil,err
	}
	return kendynet.NewStreamSocket(conn),nil,nil
}
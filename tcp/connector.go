package tcp

import (
    "net"
    "github.com/sniperHW/kendynet"
    "time"
)

type Connector struct{
	nettype string
	addr    string
	//addr *net.TCPAddr
}

func NewConnector(nettype string,addr string) (*Connector,error) {
	/*addr,err := net.ResolveTCPAddr(nettype, service)
	if nil != err {
		return nil,err
	}*/
	return &Connector{nettype:nettype,addr:addr},nil
}

func (this *Connector) Dial(timeout time.Duration) (kendynet.StreamSession,interface{},error) {
	dialer := &net.Dialer{Timeout:timeout}
	conn, err := dialer.Dial(this.nettype , this.addr)
	if err != nil {
		return nil,nil,err
	}
	return kendynet.NewStreamSocket(conn),nil,nil
}
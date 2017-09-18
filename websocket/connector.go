package websocket

import (
	"net/url"
	"net/http"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"time"
)


type Connector struct{
	dialer 		 *gorilla.Dialer
	u       	  url.URL
	requestHeader http.Header
}

func NewConnector(url url.URL,requestHeader http.Header,dialer *gorilla.Dialer) (*Connector,error) {
	if nil == dialer {
		return nil,ErrWSClientInvaildDialer
	}
	client := Connector{}
	client.u = url
	client.dialer = dialer
	client.requestHeader = requestHeader
	return &client,nil
}

func (this *Connector) Dial(timeout time.Duration) (kendynet.StreamSession,interface{},error) {
	this.dialer.HandshakeTimeout = timeout
	c, response, err := this.dialer.Dial(this.u.String(), nil)
	if err != nil {
		return nil,nil,err
	}
	return NewWSSocket(c),response,nil	
}
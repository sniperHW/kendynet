package websocket

import (
	"net/url"
	"net/http"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
)


type WSClient struct{
	dialer 		 *gorilla.Dialer
	u       	  url.URL
	requestHeader http.Header
}

func NewClient(url url.URL,requestHeader http.Header,dialer *gorilla.Dialer) (*WSClient,error) {
	if nil == dialer {
		return nil,ErrWSClientInvaildDialer
	}
	client := WSClient{}
	client.u = url
	client.dialer = dialer
	client.requestHeader = requestHeader
	return &client,nil
}

func (this *WSClient) Dial() (kendynet.StreamSession,interface{},error) {
	c, response, err := this.dialer.Dial(this.u.String(), nil)
	if err != nil {
		return nil,nil,err
	}
	return NewWSSocket(c),response,nil	
}
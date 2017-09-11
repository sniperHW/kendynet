package kendynet

import (
	"net/url"
	"github.com/gorilla/websocket"
	"net/http"
	"fmt"
)

var (
    ErrWSClientInvaildDialer = fmt.Errorf("dialer == nil")
)

type WSClient struct{
	dialer 		 *websocket.Dialer
	u       	  url.URL
	requestHeader http.Header
}

func NewWSClient(url url.URL,requestHeader http.Header,dialer *websocket.Dialer) (*WSClient,error) {
	if nil == dialer {
		return nil,ErrWSClientInvaildDialer
	}
	client := WSClient{}
	client.u = url
	client.dialer = dialer
	client.requestHeader = requestHeader
	return &client,nil
}

func (this *WSClient) Dial() (StreamSession,interface{},error) {
	c, response, err := this.dialer.Dial(this.u.String(), nil)
	if err != nil {
		return nil,nil,err
	}
	return NewWSSocket(c),response,nil	
}
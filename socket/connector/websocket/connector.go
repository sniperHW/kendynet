package websocket

import (
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket"
	"net/http"
	"net/url"
	"time"
)

type Connector struct {
	dialer        *gorilla.Dialer
	u             url.URL
	requestHeader http.Header
}

func New(url url.URL, requestHeader http.Header, dialer ...*gorilla.Dialer) (*Connector, error) {

	client := &Connector{
		u:             url,
		requestHeader: requestHeader,
	}

	if len(dialer) > 0 {
		client.dialer = dialer[0]
	} else {
		client.dialer = gorilla.DefaultDialer
	}

	return client, nil
}

func (this *Connector) Dial(timeout time.Duration) (kendynet.StreamSession, *http.Response, error) {
	this.dialer.HandshakeTimeout = timeout
	c, response, err := this.dialer.Dial(this.u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	return socket.NewWSSocket(c), response, nil
}

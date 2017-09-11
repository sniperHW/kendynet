package kendynet

import (
//	"fmt"
)

/*
var (
    ErrInvaildOnConnectedCB = fmt.Errorf("onConnected == nil")
)
*/

type Client interface {
	Dial()(StreamSession,interface{},error)
}
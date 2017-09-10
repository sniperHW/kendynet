package kendynet

import (
	"fmt"
)

var (
    ErrServerStarted = fmt.Errorf("Server already started")
    ErrInvaildNewClientCB = fmt.Errorf("onNewClient == nil")
)

type Server interface {
	Close()
	Start(func(StreamSession)) error
}
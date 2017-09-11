package kendynet

import (
	"fmt"
)

var (
    ErrServerStarted 		   = fmt.Errorf("Server already started")
    ErrInvaildNewClientCB      = fmt.Errorf("onNewClient == nil")
	ErrBuffMaxSizeExceeded     = fmt.Errorf("bytebuffer: Max Buffer Size Exceeded")
	ErrBuffInvaildAgr          = fmt.Errorf("bytebuffer: Invaild Idx or size")
    ErrSocketClose       	   = fmt.Errorf("socket close")
    ErrSendTimeout             = fmt.Errorf("send timeout")
    ErrStarted                 = fmt.Errorf("already started")
    ErrInvaildBuff             = fmt.Errorf("buff is nil")
    ErrNoOnPacket              = fmt.Errorf("onPacket == nil")
    ErrNoReceiver              = fmt.Errorf("receiver == nil")
    ErrInvaildObject           = fmt.Errorf("object == nil")
    ErrInvaildEncoder          = fmt.Errorf("encoder == nil")
    ErrInvaildWSMessage        = fmt.Errorf("invaild websocket message")
    ErrWSPeerClose             = fmt.Errorf("receive websocket peer close")
    ErrWSInvaildUpgrader       = fmt.Errorf("upgrader == nil")
)
package websocket

import (
    "fmt"
)

var (
    ErrInvaildWSMessage      = fmt.Errorf("invaild websocket message")
    ErrWSPeerClose           = fmt.Errorf("receive websocket peer close")
    ErrWSInvaildUpgrader     = fmt.Errorf("upgrader == nil")
    ErrWSClientInvaildDialer = fmt.Errorf("dialer == nil")
)
package websocket

import (
    "fmt"
)

var (
    ErrInvaildWSMessage      = fmt.Errorf("invaild websocket message")
    ErrWSInvaildUpgrader     = fmt.Errorf("upgrader == nil")
    ErrWSClientInvaildDialer = fmt.Errorf("dialer == nil")
)
package kendynet

import (
	"time"
)

type Connector interface {
	Dial(timeout time.Duration) (StreamSession,interface{},error)
}
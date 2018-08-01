package event

import (
	"github.com/sniperHW/kendynet/golog"
	"sync/atomic"
	"fmt"
)

var logger *golog.Logger
var is_init int32

func InitLogger(out *golog.OutputLogger,name ...string) {
	if atomic.CompareAndSwapInt32(&is_init, 0, 1) {
		var fullname string
		if len(name) > 0 {
			fullname = fmt.Sprintf("%s",name[0])
		} else {
			fullname = "event"
		}
		logger = golog.New(fullname,out)
		logger.Debugf("%s logger init",fullname)
	}
}

func Errorf(format string, v ...interface{}) {
	if nil != logger {
		logger.Errorf(format, v...)
	}
}

func Errorln(v ...interface{}) {
	if nil != logger {
		logger.Errorln(v...)
	}
}
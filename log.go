package kendynet

import (
	"github.com/sniperHW/kendynet/golog"
)

func NewLog(dir,name string) *golog.Logger {

	out := golog.NewOutputLogger(dir,name,1024*1024*100)

	if out != nil {
		return golog.New(name,out)
	} else {
		return nil
	}
}




package kendynet

import (
	"github.com/davyxu/golog"
	"os"
	"time"
	"fmt"
)

var Logger *golog.Logger

func itoa(buf *[]byte, i int, wid int) {
	var u uint = uint(i)
	if u == 0 && wid <= 1 {
		*buf = append(*buf, '0')
		return
	}

	// Assemble decimal in reverse order.
	var b [32]byte
	bp := len(b)
	for ; u > 0 || wid > 0; u /= 10 {
		bp--
		wid--
		b[bp] = byte(u%10) + '0'
	}
	*buf = append(*buf, b[bp:]...)
}

func NewLog(name string) *golog.Logger {
	Logger := golog.New(name)
	t := time.Now()
	buf := make([]byte,0)
	year, month, day := t.Date()
	itoa(&buf, year, 4)
	buf = append(buf, '-')
	itoa(&buf, int(month), 2)
	buf = append(buf, '-')
	itoa(&buf, day, 2)
	buf = append(buf, ' ')

	hour, min, sec := t.Clock()
	itoa(&buf, hour, 2)
	buf = append(buf, ':')
	itoa(&buf, min, 2)
	buf = append(buf, ':')
	itoa(&buf, sec, 2)

	var filename string

	if nil != os.Mkdir("log",os.ModePerm) {
		filename = fmt.Sprintf("./log/%s[%s].log",name,string(buf))
	} else {
		filename = fmt.Sprintf("%s[%s].log",name,string(buf))
	}

	golog.SetOutputLogger("kendynet",filename)

	return 	Logger
}

func init() {
	Logger = NewLog("kendynet")
	Logger.Infoln("kendynet log init")	
}
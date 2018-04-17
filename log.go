package kendynet

import (
	"github.com/sniperHW/kendynet/golog"
	"os"
	"time"
	"fmt"
)

var folder string     = "log"  //默认日誌根目錄
var logPrefix string  = ""     //日誌文件前綴

func SetLogFolder(_folder string) {
	folder = _folder
}

func GetLogFolder() string {
	return folder
}

func SetLogPrefix(_logPrefix string) {
	logPrefix = _logPrefix
}

func GetLogPrefix() string {
	return logPrefix
}

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
	logger := golog.New(name)
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
	buf = append(buf, '.')
	itoa(&buf, min, 2)
	buf = append(buf, '.')
	itoa(&buf, sec, 2)

	var filename string

	if nil == os.MkdirAll(folder,os.ModePerm) {
		filename = fmt.Sprintf("%s/%s[%s][%d].log",folder,name,string(buf),os.Getpid())
	} else {
		filename = fmt.Sprintf("%s[%s][%s].log",name,string(buf),os.Getpid())
	}

	golog.SetOutputLogger(name,filename)

	return 	logger
}




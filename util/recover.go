package util

import( 
	"fmt"
	"runtime"
)

func RecoverAndPrintStack() {
	if r := recover(); r != nil {
		buf := make([]byte, 65535)
		l := runtime.Stack(buf, false)
		fmt.Printf("%s\n",fmt.Errorf("%v: %s", r, buf[:l]).Error())
	}
}
package main 


import(
	"github.com/sniperHW/kendynet/timer"
	"os"
	"fmt"
	"time"
	"os/signal"
	"syscall"
)


func main() {

	timer.New(1 * time.Second,true,nil,func (timer timer.TimerID) {
		fmt.Println("timer")
	})


   	c := make(chan os.Signal) 
    signal.Notify(c, syscall.SIGINT)  //监听指定信号
    _ = <-c //阻塞直至有信号传入

}
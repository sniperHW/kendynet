package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/timer"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	i := 0

	timer.Repeat(1*time.Second, nil, func(timer *timer.Timer) {
		i++
		fmt.Println("Repeat timer", i)
		if i > 10 {
			timer.Cancel()
		}
	})

	timer.Once(1*time.Second, nil, func(_ *timer.Timer) {
		fmt.Println("Once timer")
	})

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT) //监听指定信号
	_ = <-c                          //阻塞直至有信号传入

}

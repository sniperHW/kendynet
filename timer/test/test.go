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

	timer.Repeat(1*time.Second, nil, func(timer *timer.Timer, ctx interface{}) {
		i++
		fmt.Println("Repeat timer", i)
		if i == 10 {
			timer.Cancel()
		}
	}, nil)

	timer.Once(12*time.Second, nil, func(_ *timer.Timer, ctx interface{}) {
		fmt.Println("Once timer")
	}, nil)

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT) //监听指定信号
	_ = <-c                          //阻塞直至有信号传入

}

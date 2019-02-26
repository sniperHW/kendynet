package main

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/golog"
)

//var eventQueue *event.EventQueue

func testQueueMode() {

	fmt.Println("-------------------testQueueMode-----------------")

	handler := event.NewEventHandler()

	handler.Register("queue", func(_ event.Handle) {
		fmt.Println("handler1")
		handler.Clear("queue")
	})

	handler.Register("queue", func(_ event.Handle) {
		fmt.Println("handler2")
	})

	handler.Register("queue", func(_ event.Handle) {
		fmt.Println("handler3")
	})

	/*
	 * 所有注册的处理器将按注册顺序依次执行
	 */

	handler.Emit("queue")

	fmt.Println("again")

	//再次触发事件，此时handler2已经被删除，所以不会再次被调用

	handler.Emit("queue")

}

func testQueueOnceMode() {

	fmt.Println("-------------------testQueueOnceMode-----------------")

	handler := event.NewEventHandler()

	handler.Register("queue", func(_ event.Handle, msg ...interface{}) {
		fmt.Println("handler1", msg[0])
	})

	handler.RegisterOnce("queue", func(_ event.Handle) {
		fmt.Println("handler2")
	})

	handler.Register("queue", func(_ event.Handle) {
		fmt.Println("handler3")
	})

	/*
	* 所有注册的处理器将按注册顺序依次执行
	 */

	handler.Emit("queue", "hello")

	fmt.Println("again")

	//再次触发事件，因为handler2被注册为只触发一次，此时handler2已经被删除，所以不会再次被调用

	handler.Emit("queue", "world")

}

func main() {

	outLogger := golog.NewOutputLogger("log", "kendynet", 1024*1024*1000)
	kendynet.InitLogger(golog.New("rpc", outLogger))

	testQueueMode()

	testQueueOnceMode()

	//eventQueue.Close()

	//eventQueue.Run()

}

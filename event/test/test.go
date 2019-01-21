package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/event"
)

var eventQueue *event.EventQueue

func testQueueMode() {

	fmt.Println("-------------------testQueueMode-----------------")

	handler := event.NewEventHandler(eventQueue)

	handler.Register("queue", false, func() {
		fmt.Println("handler1")
	})

	var h *event.Handle

	h = handler.Register("queue", false, func() {
		fmt.Println("handler2")
		handler.Remove(h)
	})

	handler.Register("queue", false, func() {
		fmt.Println("handler3")
	})

	/*
	* 所有注册的处理器将按注册顺序依次执行
	 */

	handler.Emit("queue")

	fmt.Println("again")

	//再次触发事件，此时handler2已经被删除，所以不会再次被调用

	handler.Emit("queue")

	fmt.Println("")

}

func testQueueOnceMode() {

	fmt.Println("-------------------testQueueOnceMode-----------------")

	handler := event.NewEventHandler(eventQueue)

	handler.Register("queue", false, func() {
		fmt.Println("handler1")
	})

	handler.Register("queue", true, func() {
		fmt.Println("handler2")
	})

	handler.Register("queue", false, func() {
		fmt.Println("handler3")
	})

	/*
	* 所有注册的处理器将按注册顺序依次执行
	 */

	handler.Emit("queue")

	//再次触发事件，因为handler2被注册为只触发一次，此时handler2已经被删除，所以不会再次被调用

	handler.Emit("queue")

}

func main() {

	eventQueue = event.NewEventQueue()

	testQueueMode()

	//testQueueOnceMode()

	eventQueue.Close()

	eventQueue.Run()

}

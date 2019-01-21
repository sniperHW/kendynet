package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/event"
)

var eventQueue *event.EventQueue

func testQueueMode() {

	fmt.Println("-------------------testQueueMode-----------------")

	var err error

	handler := event.NewEventHandler(eventQueue)

	_, err = handler.Register("queue", false, func() {
		fmt.Println("handler1")
	})

	if err != nil {
		fmt.Println(err)
	}

	var h *event.Handle

	h, err = handler.Register("queue", false, func() {
		fmt.Println("handler2")
		handler.Remove(h)
	})

	if err != nil {
		fmt.Println(err)
	}

	_, err = handler.Register("queue", false, func() {
		fmt.Println("handler3")
	})

	if err != nil {
		fmt.Println(err)
	}

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

	var err error

	handler := event.NewEventHandler(eventQueue)

	_, err = handler.Register("queue", false, func() {
		fmt.Println("handler1")
	})

	if err != nil {
		fmt.Println(err)
	}

	_, err = handler.Register("queue", true, func() {
		fmt.Println("handler2")
	})

	if err != nil {
		fmt.Println(err)
	}

	_, err = handler.Register("queue", false, func() {
		fmt.Println("handler3")
	})

	if err != nil {
		fmt.Println(err)
	}

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

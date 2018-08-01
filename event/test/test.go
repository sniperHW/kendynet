package main 

import(
	"fmt"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/golog"
)


func testQueueMode() {

	fmt.Println("-------------------testQueueMode-----------------")
	
	var err error

	handler := event.NewEventHandler()
	
	_,err = handler.Register(event.Mode_queue,"queue",func () {
		fmt.Println("handler1")
	})

	if err != nil {
		fmt.Println(err)
	}

	var id event.HandlerID

	id,err = handler.Register(event.Mode_queue,"queue",func () {
		fmt.Println("handler2")
		handler.Remove("queue",id)
	})

	if err != nil {
		fmt.Println(err)
	}

	_,err = handler.Register(event.Mode_queue,"queue",func () {
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

	handler := event.NewEventHandler()
	
	_,err = handler.Register(event.Mode_queue,"queue",func () {
		fmt.Println("handler1")
	})

	if err != nil {
		fmt.Println(err)
	}


	_,err = handler.Register(event.Mode_queue | event.Mode_once,"queue",func () {
		fmt.Println("handler2")
	})

	if err != nil {
		fmt.Println(err)
	}

	_,err = handler.Register(event.Mode_queue,"queue",func () {
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

	//再次触发事件，因为handler2被注册为只触发一次，此时handler2已经被删除，所以不会再次被调用

	handler.Emit("queue")

	fmt.Println("")

}

func testExclusiveMode() {

	fmt.Println("-------------------testExclusiveMode-----------------")
	
	var err error

	handler := event.NewEventHandler()
	
	_,err = handler.Register(event.Mode_queue,"exclusive",func () {
		fmt.Println("handler1")
	})

	if err != nil {
		fmt.Println(err)
	}

	_,err = handler.Register(event.Mode_queue,"exclusive",func () {
		fmt.Println("handler2")
	})

	if err != nil {
		fmt.Println(err)
	}

	_,err = handler.Register(event.Mode_exclusive | event.Mode_once ,"exclusive",func () {
		fmt.Println("handler3")
	})	

	if err != nil {
		fmt.Println(err)
	}

	//因为handler3以排它模式注册，所以只会执行handler3
	handler.Emit("exclusive")

	fmt.Println("again")

	//再次触发事件，因为handler3被注册为只触发一次，此时handler3已经被删除，所以不会再次被调用

	handler.Emit("exclusive")

	fmt.Println("\n")

}


func testRecursive() {
	handler := event.NewEventHandler()
	
	handler.Register(event.Mode_queue,"recursive",func () {
		fmt.Println("handler1")
		handler.Emit("recursive")
	})	

	handler.Emit("recursive")
}

func main() {
	outLogger := golog.NewOutputLogger("log","event",1024*1024*1000)
	event.InitLogger(outLogger)

	testQueueMode()

	testQueueOnceMode()

	testExclusiveMode()

	testRecursive()
}
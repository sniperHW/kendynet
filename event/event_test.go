package event

//go test -covermode=count -v -run=.
import (
	"fmt"
	"testing"
)

func testQueueMode() {

	fmt.Println("-------------------testQueueMode-----------------")

	handler := NewEventHandler()

	handler.Register("queue", func() {
		fmt.Println("handler1")
		handler.Clear("queue")
		handler.Register("queue", func() {
			fmt.Println("handler11")
		})
	})

	handler.Register("queue", func() {
		fmt.Println("handler2")
	})

	handler.Register("queue", func() {
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

	handler := NewEventHandler()

	handler.Register("queue", func(msg ...interface{}) {
		fmt.Println("handler1", msg[0])
	})

	handler.RegisterOnce("queue", func() {
		fmt.Println("handler2")
	})

	handler.Register("queue", func() {
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

func testUseEventQueue() {
	queue := NewEventQueue()
	go queue.Run()

	handler := NewEventHandler(queue)

	handler.Register("queue", func() {
		fmt.Println("handler1")
		queue.Close()
	})

	handler.Emit("queue")

}

func TestEvent(t *testing.T) {
	testQueueMode()

	testQueueOnceMode()

	testUseEventQueue()
}

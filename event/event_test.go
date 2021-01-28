package event

//go test -covermode=count -v -coverprofile=coverage.out -run=TestEvent
//go tool cover -html=coverage.out
import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEventQueue(t *testing.T) {
	fmt.Println("-------------------testUseEventQueue-----------------")

	queue := NewEventQueue(1)

	handler := NewEventHandler()

	handler.Register("queue", func(v int) {
		fmt.Println("handler1", v)
	})

	handler.EmitToEventQueue(EventQueueParam{
		Q:         queue,
		BlockMode: true,
	}, "queue", 1)

	handler.EmitToEventQueue(EventQueueParam{
		Q: queue,
	}, "queue", 1)

	handler.EmitToEventQueue(EventQueueParam{
		Q:          queue,
		FullReturn: true,
	}, "queue", 1)

	queue.PostNoWait(1, func(...interface{}) {
		fmt.Println("queue fun3")
		queue.Close()
	})

	queue.Run()

}

func TestEvent(t *testing.T) {

	{
		handler := NewEventHandler()
		handler.RegisterOnce("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler1", msg[0])
			handler.Remove(h)
		})
		handler.Emit("queue", 1)

		handler.Lock()
		slot, _ := handler.slots["queue"]
		handler.Unlock()

		assert.Equal(t, slot.l.head.nnext, &slot.l.tail)
		assert.Equal(t, &slot.l.head, slot.l.tail.pprev)

		h2 := handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler2", msg[0])
		})

		handler.RegisterOnce("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler3", msg[0])
		})

		h4 := handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler4", msg[0])
		})

		handler.Emit("queue", 1)

		assert.Equal(t, h2.nnext, (*handle)(h4))
		assert.Equal(t, h4.pprev, (*handle)(h2))

	}

	{
		fmt.Println("test1-----------------")
		handler := NewEventHandler()
		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler1", msg[0])
		})

		handler.RegisterOnce("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler2", msg[0])
		})

		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler3", msg[0])
		})

		handler.Emit("queue", 1)
		fmt.Println("again")
		handler.Emit("queue", 1)

	}

	{
		fmt.Println("test2-----------------")
		handler := NewEventHandler()
		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler1", msg[1])
		})

		h2 := handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler2", msg[1])
			handler.Remove(h)
		})

		h3 := handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler3", msg[1])
		})

		handler.Emit("queue", h2, 1)
		fmt.Println("again")
		handler.Emit("queue", h2, 1)
		fmt.Println("again")
		handler.Remove(h3)
		handler.Emit("queue", h2, 1)

	}

	{
		fmt.Println("test3-----------------")
		handler := NewEventHandler()
		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler1", msg[0])
		})

		handler.RegisterOnce("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler2", msg[0])
			handler.Register("queue", func(h Handle, msg ...interface{}) {
				fmt.Println("handler4", msg[0])
			})
			handler.Register("queue", func(h Handle, msg ...interface{}) {
				fmt.Println("handler6", msg[0])
			})

		})

		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler3", msg[0])
		})

		handler.Emit("queue", 1)
		fmt.Println("again")
		handler.Emit("queue", 1)

		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler5", msg[0])
		})
		fmt.Println("again")
		handler.Emit("queue", 1)

	}

	{
		fmt.Println("test4-----------------")
		handler := NewEventHandler()

		c := 0

		handler.Register("queue", func(h Handle, msg ...interface{}) {
			c++
			fmt.Println("handler1", msg[0], c)
			if c < 3 {
				handler.Emit("queue", 1)
			}
		})

		handler.Emit("queue", 1)
	}

	{
		fmt.Println("test5-----------------")
		handler := NewEventHandler()

		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler1", msg[0])
		})

		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler2", msg[0])
			handler.Clear("queue")
		})

		handler.Register("queue", func(h Handle, msg ...interface{}) {
			fmt.Println("handler3", msg[0])
		})

		handler.Emit("queue", 1)
		fmt.Println("again")
		handler.Emit("queue", 1)
	}

	//testQueueMode()

	//testQueueOnceMode()

	//testUseEventQueue()
}

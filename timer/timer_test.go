package timer

//go test -covermode=count -v -run=.
import (
	"fmt"
	"github.com/sniperHW/kendynet/event"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestTimer(t *testing.T) {

	{

		die := make(chan struct{})
		i := 0
		timer_ := Repeat(100*time.Millisecond, nil, func(timer_ *Timer, ctx interface{}) {
			i++
			fmt.Println("Repeat timer", i)
			if i == 10 {
				//在回调内所以Cancel返回false,但timer不会再继续执行
				assert.Equal(t, false, timer_.Cancel())
				close(die)
			}
		}, nil)

		<-die

		assert.Equal(t, i, 10)

		assert.Equal(t, timer_.Cancel(), false)

	}

	{

		queue := event.NewEventQueue()

		go queue.Run()

		die := make(chan struct{})
		i := 0
		timer_ := Repeat(100*time.Millisecond, queue, func(timer_ *Timer, ctx interface{}) {
			i++
			fmt.Println("Repeat timer", i)
			if i == 10 {
				//在回调内所以Cancel返回false,但timer不会再继续执行
				assert.Equal(t, false, timer_.Cancel())
				close(die)
			}
		}, nil)

		<-die

		assert.Equal(t, i, 10)

		assert.Equal(t, timer_.Cancel(), false)

		queue.Close()

	}

	{
		die := make(chan struct{})
		timer_ := Once(1*time.Second, nil, func(_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
			close(die)
		}, nil)

		<-die

		assert.Equal(t, timer_.Cancel(), false)
	}

	{

		timer_ := Once(1*time.Second, nil, func(_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
		}, nil)

		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, timer_.Cancel(), true)
	}

	{

		OnceWithIndex(1*time.Second, nil, func(_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
		}, nil, uint64(1))

		time.Sleep(100 * time.Millisecond)

		timer_ := GetTimerByIndex(uint64(1))

		assert.NotNil(t, timer_)

		assert.Equal(t, timer_.Cancel(), true)
	}

}

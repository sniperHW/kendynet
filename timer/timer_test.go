package timer

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go test -v -run=^$ -bench Benchmark -count 10
//go tool cover -html=coverage.out
import (
	"fmt"
	"github.com/sniperHW/kendynet/event"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func BenchmarkTimer(b *testing.B) {
	b.ReportAllocs()
	timers := make([]*Timer, b.N)
	for i := 0; i < b.N; i++ {
		t := Once(10*time.Second, func(_ *Timer, ctx interface{}) {
		}, nil)
		timers[i] = t
	}

	for _, v := range timers {
		v.Cancel()
	}
}

func BenchmarkGoTimer(b *testing.B) {
	b.ReportAllocs()
	timers := make([]*time.Timer, b.N)

	for i := 0; i < b.N; i++ {
		t := time.AfterFunc(10*time.Second, func() {

		})
		timers[i] = t
	}

	for _, v := range timers {
		v.Stop()
	}
}

func TestTimer(t *testing.T) {

	{

		assert.Nil(t, Once(100*time.Millisecond, nil, nil))

		tt := Once(100*time.Millisecond, func(timer_ *Timer, ctx interface{}) {
			fmt.Println(ctx.(int))
		}, nil)

		assert.Nil(t, tt.GetCTX())

		assert.Nil(t, GetTimerByIndex(0))

		index := uint64(1)

		OnceWithIndex(time.Second, func(timer_ *Timer, ctx interface{}) {
			CancelByIndex(index)
		}, nil, index)

	}

	{

		die := make(chan struct{})
		i := 0
		timer_ := Repeat(100*time.Millisecond, func(timer_ *Timer, ctx interface{}) {
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

		assert.Equal(t, false, timer_.Cancel())

	}

	{

		queue := event.NewEventQueue()

		go queue.Run()

		die := make(chan struct{})
		i := 0
		timer_ := Repeat(100*time.Millisecond, func(timer_ *Timer, ctx interface{}) {
			queue.PostNoWait(0, func() {
				i++
				fmt.Println("Repeat timer", i)
				if i == 10 {
					timer_.Cancel()
					close(die)
				}
			})
		}, nil)

		<-die

		assert.Equal(t, i, 10)

		assert.Equal(t, timer_.Cancel(), false)

		queue.Close()

	}

	{
		die := make(chan struct{})
		timer_ := Once(1*time.Second, func(timer_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
			assert.Equal(t, false, timer_.Cancel())
			close(die)
		}, nil)

		<-die

		assert.Equal(t, timer_.Cancel(), false)
	}

	{

		timer_ := Once(1*time.Second, func(_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
		}, nil)

		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, timer_.Cancel(), true)
	}

	{

		OnceWithIndex(1*time.Second, func(_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
		}, nil, uint64(1))

		time.Sleep(100 * time.Millisecond)

		timer_ := GetTimerByIndex(uint64(1))

		assert.NotNil(t, timer_)

		assert.Equal(t, timer_.Cancel(), true)
	}

	{

		OnceWithIndex(1*time.Second, func(_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
		}, 1, uint64(1))

		assert.Nil(t, OnceWithIndex(1*time.Second, func(_ *Timer, ctx interface{}) {
			fmt.Println("Once timer")
		}, 1, uint64(1)))

		time.Sleep(100 * time.Millisecond)

		ok, ctx := CancelByIndex(uint64(1))
		assert.Equal(t, true, ok)
		assert.Equal(t, 1, ctx.(int))

		ok, ctx = CancelByIndex(uint64(1))
		assert.Equal(t, false, ok)

	}

	{

		die := make(chan struct{})

		expect_firetime := time.Now().Unix() + 2
		var firetime int64
		timer_ := Once(5*time.Second, func(_ *Timer, ctx interface{}) {
			firetime = time.Now().Unix()
			fmt.Println("Once timer")
			close(die)
		}, nil)

		assert.Equal(t, true, timer_.ResetFireTime(2*time.Second))

		<-die

		assert.Equal(t, firetime, expect_firetime)

	}

	{

		die := make(chan struct{})

		expect_firetime := time.Now().Unix() + 5
		var firetime int64
		timer_ := Once(2*time.Second, func(_ *Timer, ctx interface{}) {
			firetime = time.Now().Unix()
			fmt.Println("Once timer")
			close(die)
		}, nil)

		assert.Equal(t, true, timer_.ResetFireTime(5*time.Second))

		<-die

		assert.Equal(t, firetime, expect_firetime)

		assert.Equal(t, false, timer_.ResetFireTime(5*time.Second))

	}

}

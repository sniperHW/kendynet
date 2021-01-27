package asyn

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go tool cover -html=coverage.out
import (
	"context"
	"fmt"
	"github.com/sniperHW/kendynet/event"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func mySleep1() int {
	fmt.Println("mySleep1 sleep")
	time.Sleep(time.Second)
	fmt.Println("mySleep1 wake")
	return 1
}

func mySleep2(s int) int {
	fmt.Println("mySleep2 sleep")
	time.Sleep(time.Second * time.Duration(s))
	fmt.Println("mySleep2 wake")
	return 2
}

func mySleep3() {
	fmt.Println("mySleep3 sleep")
	time.Sleep(time.Second * time.Duration(1))
	fmt.Println("mySleep3 wake")
}

type st struct {
	data int
}

func (this *st) fun() {
	time.Sleep(time.Second * 3)
	fmt.Println("fun", this.data)
}

func TestAsyn(t *testing.T) {
	{
		//all
		begUnix := time.Now().Unix()
		ret, err := Paralell(
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 1)
				return 1
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 2
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 3)
				return 3
			},
		).Wait()

		assert.Nil(t, err)
		assert.Equal(t, time.Now().Unix()-begUnix, int64(3))
		assert.Equal(t, len(ret), 3)
		assert.Equal(t, 1, ret[0].(int))
		assert.Equal(t, 2, ret[1].(int))
		assert.Equal(t, 3, ret[2].(int))
	}

	{
		//any
		begUnix := time.Now().Unix()
		ret, err := Paralell(
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 1)
				return 1
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 2
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 3)
				return 3
			},
		).WaitAny()

		assert.Nil(t, err)
		assert.Equal(t, time.Now().Unix()-begUnix, int64(1))
		assert.Equal(t, 1, ret.(int))
	}

	{
		//any
		begUnix := time.Now().Unix()
		ret, err := Paralell(
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 1)
				return 1
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 2
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 3)
				return 3
			},
		).WaitAny(time.Second * 4)

		assert.Nil(t, err)
		assert.Equal(t, time.Now().Unix()-begUnix, int64(1))
		assert.Equal(t, 1, ret.(int))
	}

	{
		//any
		_, err := Paralell(
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 1
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 2
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 3)
				return 3
			},
		).WaitAny(time.Second)

		assert.Equal(t, err, ErrTimeout)
	}

	{
		wg := &sync.WaitGroup{}
		begUnix := time.Now().Unix()
		_, err := Paralell(
			func(ctx context.Context) interface{} {
				defer wg.Done()
				wg.Add(1)
				for i := 1; i < 3; i++ {
					select {
					case <-ctx.Done():
						fmt.Printf("stop 1\n")
						return nil
					default:
						time.Sleep(time.Second * 2)
					}
				}
				return 1
			},
			func(ctx context.Context) interface{} {
				defer wg.Done()
				wg.Add(1)
				for i := 1; i < 3; i++ {
					select {
					case <-ctx.Done():
						fmt.Printf("stop 2\n")
						return nil
					default:
						time.Sleep(time.Second * 2)
					}
				}
				return 2
			},
			func(ctx context.Context) interface{} {
				defer wg.Done()
				wg.Add(1)
				for i := 1; i < 3; i++ {
					select {
					case <-ctx.Done():
						fmt.Printf("stop 3\n")
						return nil
					default:
						time.Sleep(time.Second * 2)
					}
				}
				return 3
			},
		).Wait(time.Second * 1)

		assert.Equal(t, time.Now().Unix()-begUnix, int64(1))
		assert.Equal(t, err, ErrTimeout)

		wg.Wait()

	}

	{
		future := Paralell(
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 1)
				return 1
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 2
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 3)
				return 3
			},
		)

		time.Sleep(time.Second * 5)

		//5秒之后等待执行结果，应该立即返回，应为在这一点所有闭包都已执行完毕
		begUnix := time.Now().Unix()
		ret, err := future.Wait()

		assert.Nil(t, err)
		assert.Equal(t, time.Now().Unix()-begUnix, int64(0))
		assert.Equal(t, len(ret), 3)
		assert.Equal(t, 1, ret[0].(int))
		assert.Equal(t, 2, ret[1].(int))
		assert.Equal(t, 3, ret[2].(int))

	}

	{
		future := Paralell(
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 1)
				return 1
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 2
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 3)
				return 3
			},
		)

		ret, err := future.Wait(time.Second * 4)

		assert.Nil(t, err)
		assert.Equal(t, len(ret), 3)
		assert.Equal(t, 1, ret[0].(int))
		assert.Equal(t, 2, ret[1].(int))
		assert.Equal(t, 3, ret[2].(int))

	}

	{
		fmt.Println("---------------")
		future := Paralell(
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 1)
				return 1
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 2)
				return 2
			},
			func(_ context.Context) interface{} {
				time.Sleep(time.Second * 3)
				return 3
			},
		)

		_, err := future.Wait(time.Second)

		assert.Equal(t, err, ErrTimeout)
	}

	{

		doCallBack1 := func(callback interface{}, args ...interface{}) {
			callback.(func(...interface{}))(args...)
		}

		c1 := make(chan struct{})
		c2 := make(chan struct{})

		wrap1 := AsynWrap(mySleep1, doCallBack1)
		wrap2 := AsynWrap(mySleep2, doCallBack1)

		wrap1(func(ret ...interface{}) {
			fmt.Println("wrap11", ret[0].(int))
			close(c1)
		}, 1)

		wrap2(func(ret ...interface{}) {
			fmt.Println("wrap21", ret[0].(int))
			close(c2)
		}, 2)

		<-c1
		<-c2

	}

	{

		doCallBack1 := func(callback interface{}, args ...interface{}) {
			callback.(func(...interface{}))(args...)
		}

		c1 := make(chan struct{})
		c2 := make(chan struct{})

		wrap1 := AsynWrap(mySleep1, doCallBack1)
		wrap2 := AsynWrap(mySleep2, doCallBack1)

		wrap1(func(ret ...interface{}) {
			fmt.Println("wrap12", ret[0].(int))
			close(c1)
		}, 1)

		wrap2(func(ret ...interface{}) {
			fmt.Println("wrap22", ret[0].(int))
			close(c2)
		}, 2)

		<-c1
		<-c2

	}

	{

		queue := event.NewEventQueue()

		doCallBack1 := func(callback interface{}, args ...interface{}) {
			queue.Post(0, callback, args...)
		}

		c1 := make(chan struct{})
		c2 := make(chan struct{})

		wrap1 := AsynWrap(mySleep1, doCallBack1)
		wrap2 := AsynWrap(mySleep2, doCallBack1)

		wrap1(func(ret int) {
			fmt.Println("wrap13", ret)
			close(c1)
		}, 1)

		wrap2(func(ret int) {
			fmt.Println("wrap23", ret)
			close(c2)
		}, 2)

		go func() {
			<-c1
			<-c2
			queue.Close()
		}()

		queue.Run()

	}

	{

		doCallBack1 := func(callback interface{}, args ...interface{}) {
			callback.(func())()
		}

		c1 := make(chan struct{})

		pool := NewRoutinePool(10)

		wrap1 := AsynWrap(mySleep3, doCallBack1, pool)

		wrap1(func() {
			fmt.Println("wrap14")
			close(c1)
		})

		<-c1

		pool.Close()

		pool.Close()

		assert.Equal(t, false, pool.AddTask(func() { fmt.Println("hello") }))

	}

}

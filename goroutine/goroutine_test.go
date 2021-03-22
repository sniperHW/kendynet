package goroutine

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go tool cover -html=coverage.out
import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestGo(t *testing.T) {
	{
		MaxCount = 2000
		ReserveCount = 1000
		var wait sync.WaitGroup
		wait.Add(10000)
		for i := 0; i < 10000; i++ {
			Go(func(i int) {
				fmt.Println(i)
				wait.Done()
			}, i)
		}

		wait.Wait()

		fmt.Println(defaultPool.totalCreateCount, defaultPool.routineCount, defaultPool.waittail)

		ch := make(chan struct{})
		OnError(func(err error) {
			fmt.Println(err)
			close(ch)
		})

		Go(func(ptr *int) {
			*ptr = 1
		}, nil)

		<-ch
	}

	{
		pool := New(Option{
			ReserveCount:    10,
			MaxCount:        10,
			MaxCurrentCount: 10,
		})
		var wait sync.WaitGroup
		wait.Add(10)
		for i := 0; i < 10; i++ {
			fmt.Println(pool.Go(func(i int) {
				time.Sleep(time.Second)
				fmt.Println(i)
				wait.Done()
			}, i))
		}

		assert.Equal(t, false, pool.Go(func(i int) {
			time.Sleep(time.Second)
			fmt.Println(i)
			wait.Done()
		}, 11))

		wait.Wait()

	}
}

func BenchmarkGoroutine(b *testing.B) {
	MaxCount = 10000
	ReserveCount = 10000
	var wait sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wait.Add(1)
		Go(func() {
			wait.Done()
		})
		wait.Wait()
	}

}

func BenchmarkRoutine(b *testing.B) {
	var wait sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wait.Add(1)
		go func() {
			wait.Done()
		}()
		wait.Wait()
	}
}

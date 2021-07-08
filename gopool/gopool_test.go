package gopool

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go test -v -run=^$ -bench Benchmark -count 10
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
		pool := New(Option{
			MaxRoutineCount:     10,
			ReserveRoutineCount: 10,
			MaxQueueSize:        -1,
		})

		var wait sync.WaitGroup

		fmt.Println("1")

		wait.Add(20)

		for i := 0; i < 20; i++ {
			pool.Go(func() {
				time.Sleep(time.Millisecond * 5)
				wait.Done()
			})
		}

		wait.Wait()

		fmt.Println("3")

		wait.Add(20)

		for i := 0; i < 20; i++ {
			pool.Go(func() {
				time.Sleep(time.Millisecond * 5)
				wait.Done()
			})
		}

		wait.Wait()

		wait.Add(120)

		for i := 0; i < 120; i++ {
			pool.Go(func() {
				time.Sleep(time.Millisecond * 5)
				wait.Done()
			})
		}

		wait.Wait()

		fmt.Println(pool.routineCount)

		pool.Close()

	}

	{
		pool := New(Option{
			MaxRoutineCount:     10,
			ReserveRoutineCount: 10,
			MaxQueueSize:        -1,
		})

		var wait sync.WaitGroup

		wait.Add(20)

		for i := 0; i < 20; i++ {
			pool.Go(func() {
				time.Sleep(time.Millisecond * 5)
				wait.Done()
			})
		}

		wait.Wait()
	}

	{
		pool := New(Option{
			MaxRoutineCount:     10,
			ReserveRoutineCount: 1,
			MaxQueueSize:        -1,
		})

		var wait sync.WaitGroup

		wait.Add(20)

		for i := 0; i < 20; i++ {
			pool.Go(func() {
				time.Sleep(time.Millisecond * 5)
				wait.Done()
			})
		}

		wait.Wait()

		time.Sleep(time.Second)

		assert.Equal(t, 1, pool.routineCount)

	}

	{
		pool := New(Option{
			MaxQueueSize: -1,
		})

		var wait sync.WaitGroup

		wait.Add(20)

		for i := 0; i < 20; i++ {
			pool.Go(func() {
				time.Sleep(time.Millisecond * 5)
				wait.Done()
			})
		}

		wait.Wait()

		assert.Equal(t, 0, pool.routineCount)

	}

	{
		pool := New(Option{
			MaxRoutineCount:     1,
			ReserveRoutineCount: 1,
			MaxQueueSize:        0,
		})

		var wait sync.WaitGroup

		wait.Add(1)

		pool.Go(func() {
			time.Sleep(time.Millisecond * 5)
			wait.Done()
		})

		assert.NotNil(t, pool.Go(func() {
			time.Sleep(time.Millisecond * 5)
			wait.Done()
		}))

		wait.Wait()

		pool.Close()

		assert.NotNil(t, pool.Go(func() {
			time.Sleep(time.Millisecond * 5)
			wait.Done()
		}))

	}

}

func BenchmarkGoroutine(b *testing.B) {
	pool := New(Option{
		MaxRoutineCount:     1024,
		ReserveRoutineCount: 1024,
		MaxQueueSize:        -1,
	})

	var wait sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wait.Add(1)
		pool.Go(func() {
			wait.Done()
		})
	}
	wait.Wait()

	pool.Close()
}

func BenchmarkRoutine(b *testing.B) {
	var wait sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wait.Add(1)
		go func() {
			wait.Done()
		}()
	}
	wait.Wait()
}

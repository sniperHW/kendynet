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

	time.Sleep(time.Second)

	assert.Equal(t, defaultPool.routineCount, defaultPool.waitCount)

	fmt.Println(defaultPool.totalCreateCount, defaultPool.routineCount, defaultPool.waitCount)

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

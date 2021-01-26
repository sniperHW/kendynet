package event

//go test -covermode=count -v -coverprofile=coverage.out -run=TestPriorityQueue
//go tool cover -html=coverage.out
import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPriorityQueue(t *testing.T) {
	{
		q := NewPriorityQueue(3)

		q.Add(0, 1)
		q.Add(0, 5)
		q.Add(0, 3)
		q.Add(0, 7)
		q.Add(0, 2)

		for q.q.count > 0 {
			_, v := q.Get()
			fmt.Println(v.(int))
		}

		fmt.Println("--------------------------------------")

		q.Add(0, 1)
		q.Add(2, 5)
		q.Add(0, 3)
		q.Add(1, 7)
		q.Add(0, 2)
		q.Add(8, 10)
		q.Add(-1, 11)

		for q.q.count > 0 {
			_, v := q.Get()
			fmt.Println(v.(int))
		}
	}

	{
		q := NewPriorityQueue(2, 5)

		q.Add(0, 1)
		q.Add(0, 2)
		q.Add(0, 3)
		q.Add(0, 4)
		q.Add(0, 5)

		assert.Equal(t, q.AddNoWait(0, 6, true), ErrQueueFull)

		assert.Equal(t, 5, q.q.count)

		c := make(chan struct{})

		go func() {
			q.Add(0, 6)
			close(c)
		}()

		time.Sleep(time.Millisecond)

		q.SetFullSize(6)

		<-c

		_, v := q.Get()
		assert.Equal(t, 1, v.(int))
		_, v = q.Get()
		assert.Equal(t, 2, v.(int))
		_, v = q.Get()
		assert.Equal(t, 3, v.(int))
		_, v = q.Get()
		assert.Equal(t, 4, v.(int))
		_, v = q.Get()
		assert.Equal(t, 5, v.(int))
		_, v = q.Get()
		assert.Equal(t, 6, v.(int))

		c = make(chan struct{})

		go func() {
			_, v := q.Get()
			assert.Equal(t, 7, v.(int))
			close(c)
		}()

		time.Sleep(time.Millisecond)

		q.Add(0, 7)

		<-c

	}

	{
		q := NewPriorityQueue(2, 5)

		q.Add(0, 1)
		q.Add(0, 2)
		q.Add(0, 3)
		q.Add(0, 4)
		q.Add(0, 5)

		assert.Equal(t, 5, q.q.count)

		c := make(chan struct{})

		go func() {
			q.Add(0, 6)
			close(c)
		}()

		time.Sleep(time.Millisecond)

		q.Get()

		<-c

	}

	{
		q := NewPriorityQueue(0, 5)

		c := make(chan struct{})

		go func() {
			q.Get()
			close(c)
		}()

		time.Sleep(time.Millisecond)

		q.AddNoWait(0, 1)

		<-c

	}

	{
		q := NewPriorityQueue(2, 5)

		c := make(chan struct{})

		q.Add(0, 1)
		q.Add(0, 2)
		q.Add(0, 3)
		q.Add(0, 4)
		q.Add(0, 5)

		go func() {
			q.Add(0, 7)
			close(c)
		}()

		time.Sleep(time.Millisecond)

		q.Close()

		<-c

	}

	{
		q := NewPriorityQueue(2, 5)

		c := make(chan struct{})

		go func() {
			q.Get()
			close(c)
		}()

		time.Sleep(time.Millisecond)

		q.Close()

		<-c

	}

	{
		q := NewPriorityQueue(2, 5)

		q.Close()
		assert.Equal(t, ErrQueueClosed, q.Add(0, 7))
		assert.Equal(t, ErrQueueClosed, q.AddNoWait(0, 7))

	}

	{
		q := NewPriorityQueue(2, 5)

		q.Add(0, 1)
		q.Add(0, 2)
		q.Add(0, 3)
		q.Add(0, 4)
		q.Add(0, 5)

		q.AddNoWait(1, 6)

		_, v := q.Get()

		assert.Equal(t, 6, v.(int))

	}

}

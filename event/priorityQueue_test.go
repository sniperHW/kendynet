package event

//go test -covermode=count -v -run=.
import (
	"fmt"
	"testing"
)

func TestPriorityQueue(t *testing.T) {
	q := NewPriorityQueue()

	q.Add(0, 1)
	q.Add(0, 5)
	q.Add(0, 3)
	q.Add(0, 7)
	q.Add(0, 2)

	for q.list.Len() > 0 {
		_, v := q.Get()
		fmt.Println(v.(int))
	}

	fmt.Println("--------------------------------------")

	q.Add(0, 1)
	q.Add(2, 5)
	q.Add(0, 3)
	q.Add(1, 7)
	q.Add(0, 2)

	for q.list.Len() > 0 {
		_, v := q.Get()
		fmt.Println(v.(int))
	}

	/*_, data = q.Get()

	for _, v := range data {
		fmt.Println(v.Value.(int))
	}*/

}

package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/util"
)

type Ele struct {
	heapIdx int
	value   uint32
}

func (this *Ele) Less(o util.HeapElement) bool {
	return this.value < o.(*Ele).value
}

func (this *Ele) GetIndex() int {
	return this.heapIdx
}

func (this *Ele) SetIndex(idx int) {
	this.heapIdx = idx
}

func main() {
	heap := util.NewMinHeap(10)

	ele1 := &Ele{value: 10}
	ele2 := &Ele{value: 20}
	ele3 := &Ele{value: 5}
	/*ele4 := &Ele{value: 9}
	ele5 := &Ele{value: 6}
	ele6 := &Ele{value: 5}
	ele7 := &Ele{value: 2}
	ele8 := &Ele{value: 1}
	ele9 := &Ele{value: 8}
	ele10 := &Ele{value: 3}*/

	heap.Insert(ele1)
	heap.Insert(ele2)
	heap.Insert(ele3)

	fmt.Println(ele3.GetIndex())

	ele3.value = 100
	heap.Fix(ele3)
	fmt.Println(ele3.GetIndex())
	/*heap.Insert(ele4)
	heap.Insert(ele5)
	heap.Insert(ele6)
	heap.Insert(ele7)
	heap.Insert(ele8)
	heap.Insert(ele9)
	heap.Insert(ele10)

	heap.Remove(ele3)

	heap.Insert(&Ele{value: 100})
	heap.Insert(&Ele{value: 97})
	heap.Insert(&Ele{value: 66})
	heap.Insert(&Ele{value: 32})
	heap.Insert(&Ele{value: 71})*/

	//ele7.value = 199
	//heap.Insert(ele7)

	for {
		if e := heap.PopMin(); nil == e {
			break
		} else {
			fmt.Println(e)
		}

	}

}

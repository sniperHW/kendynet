package main 


import (
	"sync"
	"fmt"
	//"time"
)

type st struct {
	a int
	b int
}

func main() {
	p := sync.Pool{New:func()interface{}{
		return &st{a:0,b:0}
	}}

	st1 := p.Get().(*st)
	st1.a = 1
	st1.b = 2
	p.Put(st1)

	st2 := p.Get().(*st)
	//st1 = p.Get().(*st)

	fmt.Println(st2)



}
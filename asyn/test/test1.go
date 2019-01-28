package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/asyn"
	"github.com/sniperHW/kendynet/event"
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

type st struct {
	data int
}

func (this *st) fun() {
	time.Sleep(time.Second * 3)
	fmt.Println("fun", this.data)
}

func main() {

	//设置go程池,将同步任务交给go程池执行
	asyn.SetRoutinePool(asyn.NewRoutinePool(1024))

	queue := event.NewEventQueue()
	s := st{data: 100}

	wrap1 := asyn.AsynWrap(queue, mySleep1)
	wrap2 := asyn.AsynWrap(queue, mySleep2)
	wrap3 := asyn.AsynWrap(queue, s.fun)

	wrap1(func(ret []interface{}) {
		fmt.Println(ret[0].(int))
	})

	wrap2(func(ret []interface{}) {
		fmt.Println(ret[0].(int))
	}, 2)

	wrap3(func(ret []interface{}) {
		fmt.Println("st.fun callback")
		queue.Close()
	})

	queue.Run()
}

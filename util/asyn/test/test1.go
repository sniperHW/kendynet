package main 

import(
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util/asyn"
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
	fmt.Println("fun",this.data)
} 

func main() {
	queue := kendynet.NewEventQueue()
	s := st{data:100}

	caller1 := asyn.AsynWrap(queue,mySleep1)
	caller2 := asyn.AsynWrap(queue,mySleep2)
	caller3 := asyn.AsynWrap(queue,s.fun)

	caller1.Call(func(ret []interface{}) {
		fmt.Println(ret[0].(int))
	})

	caller2.Call(func(ret []interface{}) {
		fmt.Println(ret[0].(int))
	},2)

	caller3.Call(func(ret []interface{}) {
		fmt.Println("st.fun callback")
		queue.Close()
	})


	queue.Run()
}







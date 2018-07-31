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

func main() {
	queue := kendynet.NewEventQueue()
	caller1 := asyn.AsynWrap(queue,mySleep1)
	caller2 := asyn.AsynWrap(queue,mySleep2)

	caller1.Call(func(ret []interface{}) {
		fmt.Println(ret[0].(int))
	})

	caller2.Call(func(ret []interface{}) {
		fmt.Println(ret[0].(int))
		queue.Close()
	},2)

	queue.Run()
}







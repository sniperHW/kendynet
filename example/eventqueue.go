package main

import(
	"fmt"
	"github.com/sniperHW/kendynet/event"
)


func main() {

	evQueue := event.NewEventQueue()

	go func() {
		evQueue.PostNoWait(func() {
			fmt.Println("no argument")
		})

		evQueue.PostNoWait(func(args []interface{}) {
			fmt.Printf("2 argument %d %d\n",args[0].(int),args[1].(int))
		},1,2)

		evQueue.Close()
	}()

	evQueue.Run()

}

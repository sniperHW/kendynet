package main

import(
	"fmt"
	"github.com/sniperHW/kendynet"
)


func main() {

	evQueue := kendynet.NewEventQueue()

	go func() {
		evQueue.Post(func() {
			fmt.Println("no argument")
		})

		evQueue.Post(func(args []interface{}) {
			fmt.Printf("2 argument %d %d\n",args[0].(int),args[1].(int))
		},1,2)

		evQueue.Close()
	}()

	evQueue.Run()

}

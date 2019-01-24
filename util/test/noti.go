package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/util"
)

func main() {

	noti := util.NewNotifyer()

	noti.Notify()
	fmt.Println("Notify")
	noti.Notify()
	fmt.Println("Notify")

	noti.Wait()
	fmt.Println("Wait")

	//should panic here
	noti.Wait()
	fmt.Println("Wait")

}

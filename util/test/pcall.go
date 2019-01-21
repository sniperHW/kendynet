package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/util"
)

func f1() {
	var ptr *int
	fmt.Println("f1", *ptr)
}

func f2(a int) (int, int) {
	fmt.Println("f2", a)
	return 1, 2
}

func main() {
	var err error
	err, _ = util.PCall(f1)
	if nil != err {
		fmt.Println("error on call f1\n", err)
	}

	err, ret := util.PCall(f2, 1)
	if nil == err {
		fmt.Println(ret)
	}
}

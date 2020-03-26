package main

import (
	"fmt"
	"github.com/sniperHW/kendynet/util"
)

type t struct {
}

func f1() {
	var ptr *int
	fmt.Println("f1", *ptr)
	return
}

func f2(a int, b *t) (int, int, *t) {
	fmt.Println("f2", a, b)
	return 1, 2, nil
}

func main() {
	var err error
	_, err = util.ProtectCall(f1)
	if nil != err {
		fmt.Println("error on call f1\n", err)
	}

	ret, err := util.ProtectCall(f2, 1, nil)
	if nil == err {
		fmt.Println(ret, ret[2].(*t) == nil)
	} else {
		fmt.Println("error on call f2\n", err)
	}
}

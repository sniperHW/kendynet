package socket

/*
package main

import (
        "log"
        "runtime"
        "time"
)

type I interface {
        Find()
}

type Base struct {
        child I
}

type Road struct {
        *Base
}

func (*Road) Find() {

}

func findRoad(r *Road) {
        log.Println("road:", *r)
}

func entry() {
        var rd Road = Road{}
        r := &rd
        rd.Base = &Base{child: r}
        runtime.SetFinalizer(r, findRoad)
}

func main() {

        entry()

        for i := 0; i < 10; i++ {
                time.Sleep(time.Second)
                runtime.GC()
        }

}*/

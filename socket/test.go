package socket

/*package main

import (
        "log"
        "runtime"
        "time"
)

type Base struct {
        child interface{}
}

type Road struct {
        Base
}

func findRoad(r *Road) {
        log.Println("road:", *r)
}

func entry() {
        var rd Road = Road{}
        r := &rd
        rd.child = r
        runtime.SetFinalizer(r, findRoad)
}

func main() {

        entry()

        for i := 0; i < 10; i++ {
                time.Sleep(time.Second)
                runtime.GC()
        }

}*/

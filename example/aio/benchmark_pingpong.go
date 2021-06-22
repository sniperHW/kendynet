package main

import (
	"fmt"
	"github.com/sniperHW/goaio"
	"github.com/sniperHW/kendynet/timer"
	"net"
	_ "net/http/pprof"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

func server(service string) {

	clientcount := int32(0)
	bytescount := int32(0)
	packetcount := int32(0)

	timer.Repeat(time.Second, func(_ *timer.Timer, ctx interface{}) {
		tmp1 := atomic.LoadInt32(&bytescount)
		tmp2 := atomic.LoadInt32(&packetcount)
		atomic.StoreInt32(&bytescount, 0)
		atomic.StoreInt32(&packetcount, 0)
		fmt.Printf("clientcount:%d,transrfer:%d KB/s,packetcount:%d\n", atomic.LoadInt32(&clientcount), tmp1/1024, tmp2)
	}, nil)

	tcpAddr, err := net.ResolveTCPAddr("tcp", service)
	if err != nil {
		panic(err.Error())
	}

	ln, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		panic(err.Error())
	}

	aioService := goaio.NewAIOService(1)

	go func() {
		for {
			res, ok := aioService.GetCompleteStatus()
			if !ok {
				return
			} else if nil != res.Err {
				fmt.Println("got error", res.Err)
				atomic.AddInt32(&clientcount, -1)
				res.Conn.Close(res.Err)
			} else if res.Context.(rune) == 'r' {
				atomic.AddInt32(&bytescount, int32(res.Bytestransfer))
				atomic.AddInt32(&packetcount, int32(1))
				res.Conn.Send('w', res.Buff[:res.Bytestransfer], -1)
			} else {
				res.Conn.Recv('r', res.Buff[:cap(res.Buff)], time.Second*5)
			}
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		atomic.AddInt32(&clientcount, 1)

		c, _ := aioService.CreateAIOConn(conn, goaio.AIOConnOption{})

		c.Recv('r', make([]byte, 1024*4), time.Second*5)

	}
}

func client(service string, count int) {

	aioService := goaio.NewAIOService(1)

	go func() {
		for {
			res, ok := aioService.GetCompleteStatus()
			if !ok {
				return
			} else if nil != res.Err {
				fmt.Println("go error", res.Err)
				res.Conn.Close(res.Err)
			} else if res.Context.(rune) == 'r' {
				res.Conn.Send('w', res.Buff[:res.Bytestransfer], -1)
			} else {
				res.Conn.Recv('r', res.Buff[:cap(res.Buff)], -1)
			}
		}
	}()

	for i := 0; i < count; i++ {
		conn, err := net.Dial("tcp", service)
		if err != nil {
			panic(err)
		}

		c, _ := aioService.CreateAIOConn(conn, goaio.AIOConnOption{})

		sendbuff := make([]byte, 4096)

		c.Send('w', sendbuff, -1)

	}
}

func main() {

	if len(os.Args) < 3 {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}

	mode := os.Args[1]

	if !(mode == "server" || mode == "client" || mode == "both") {
		fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
		return
	}

	service := os.Args[2]

	sigStop := make(chan bool)

	if mode == "server" || mode == "both" {
		go server(service)
	}

	if mode == "client" || mode == "both" {
		if len(os.Args) < 4 {
			fmt.Printf("usage ./pingpong [server|client|both] ip:port clientcount\n")
			return
		}
		connectioncount, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Printf(err.Error())
			return
		}
		//让服务器先运行
		time.Sleep(10000000)
		go client(service, connectioncount)

	}

	_, _ = <-sigStop

	return

}

package main


import(
	"time"
	"github.com/sniperHW/kendynet/util/asyn"
	"github.com/sniperHW/kendynet/util"	
	"github.com/sniperHW/kendynet"
	"fmt"
)

func test1(){
	fmt.Printf("test1()\n")
	ret,err := asyn.Paralell(
		func()interface{}{
			time.Sleep(time.Second * 1)
			return 1
		},
		func()interface{}{
			time.Sleep(time.Second * 2)
			return 2
		},
		func()interface{}{
			time.Sleep(time.Second * 3)
			return 3
		},				
	).Wait()

	//3秒后返回
	if nil == err {
		size := len(ret)
		for i := 0; i < size; i++ {
			fmt.Printf("%d\n",ret[i].(int))
		}
	}
}

func test2() {
	fmt.Printf("test2()\n")
	ret,err := asyn.Paralell(
		func()interface{}{
			time.Sleep(time.Second * 1)
			return 1
		},
		func()interface{}{
			time.Sleep(time.Second * 2)
			return 2
		},
		func()interface{}{
			time.Sleep(time.Second * 3)
			return 3
		},				
	).WaitAny()

	//1秒后返回

	if nil == err {
		fmt.Printf("%d\n",ret.(int))
	}
}

func test3(){
	fmt.Printf("test3()\n")	
	ret,err := asyn.Paralell(
		func()interface{}{
			time.Sleep(time.Second * 1)
			return 1
		},
		func()interface{}{
			time.Sleep(time.Second * 2)
			return 2
		},
		func()interface{}{
			time.Sleep(time.Second * 3)
			return 3
		},				
	).Wait(500)

	//500毫秒后返回等待超时错误

	if nil == err {
		size := len(ret)
		for i := 0; i < size; i++ {
			fmt.Printf("%d\n",ret[i].(int))
		}
	}else{
		fmt.Printf("%s\n",err.Error())
	}
}

func test4(){
	fmt.Printf("test4()\n")	
	future := asyn.Paralell(
		func()interface{}{
			time.Sleep(time.Second * 1)
			return 1
		},
		func()interface{}{
			time.Sleep(time.Second * 2)
			return 2
		},
		func()interface{}{
			time.Sleep(time.Second * 3)
			return 3
		},				
	)


	time.Sleep(time.Second * 5)

	//5秒之后等待执行结果，应该立即返回，应为在这一点所有闭包都已执行完毕
	ret,err := future.Wait()

	if nil == err {
		size := len(ret)
		for i := 0; i < size; i++ {
			fmt.Printf("%d\n",ret[i].(int))
		}
	}
}

func main() {

	kendynet.Logger.Debugf(util.FormatFileLine("start\n"))

	test1()
	test2()
	test3()
	test4()

}
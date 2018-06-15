package main


import(
	"time"
	"github.com/sniperHW/kendynet/util/asyn"
	"fmt"
	"context"
	"sync"
)

func test1(){
	fmt.Printf("test1\n")
	ret,err := asyn.Paralell(
		func(_ context.Context)interface{}{
			time.Sleep(time.Second * 1)
			return 1
		},
		func(_ context.Context)interface{}{
			time.Sleep(time.Second * 2)
			return 2
		},
		func(_ context.Context)interface{}{
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
	fmt.Printf("test1 done\n")
}

func test2() {
	fmt.Printf("test2\n")
	ret,err := asyn.Paralell(
		func(_ context.Context)interface{}{
			time.Sleep(time.Second * 1)
			return 1
		},
		func(_ context.Context)interface{}{
			time.Sleep(time.Second * 2)
			return 2
		},
		func(_ context.Context)interface{}{
			time.Sleep(time.Second * 3)
			return 3
		},				
	).WaitAny()

	//1秒后返回

	if nil == err {
		fmt.Printf("%d\n",ret.(int))
	}
	fmt.Printf("test2 done\n")
}

func test3(){
	fmt.Printf("test3\n")	
	wg := &sync.WaitGroup{}
	ret,err := asyn.Paralell(
		func(done context.Context)interface{}{
			defer wg.Done()
			wg.Add(1)
			for i := 1 ; i < 3 ; i++ {
        		select {
        			case <-done.Done():
            			fmt.Printf("stop 1\n")
            			return nil
        			default:
						time.Sleep(time.Second * 1)
					}
				}
			return 1
		},
		func(done context.Context)interface{}{
			defer wg.Done()
			wg.Add(1)
			for i := 1 ; i < 3 ; i++ {
        		select {
        			case <-done.Done():
            			fmt.Printf("stop 2\n")
            			return nil
        			default:
						time.Sleep(time.Second * 1)
					}
				}
			return 2
		},
		func(done context.Context)interface{}{
			defer wg.Done()
			wg.Add(1)
			for i := 1 ; i < 3 ; i++ {
        		select {
        			case <-done.Done():
            			fmt.Printf("stop 3\n")
            			return nil
        			default:
						time.Sleep(time.Second * 1)
					}
				}
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

	wg.Wait()


	fmt.Printf("test3 done\n")

}

func test4(){
	fmt.Printf("test4\n")	
	future := asyn.Paralell(
		func(_ context.Context)interface{}{
			time.Sleep(time.Second * 1)
			return 1
		},
		func(_ context.Context)interface{}{
			time.Sleep(time.Second * 2)
			return 2
		},
		func(_ context.Context)interface{}{
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

	fmt.Printf("test4 done\n")
}

func main() {

	test1()
	test2()
	test3()
	test4()

}
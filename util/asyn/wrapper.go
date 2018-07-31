package asyn


/*
*  将同步接函数调用转换成基于回调的接口
*/ 

import(
	"github.com/sniperHW/kendynet"
	"reflect"
)

type caller struct {
	queue     *kendynet.EventQueue
	oriFunc	   reflect.Value
}

func (this *caller) Call(callback func([]interface{}),args ...interface{}) {
	go func () {
		in := []reflect.Value{}
		for _,v := range(args) {
			in = append(in,reflect.ValueOf(v))
		}
		out := this.oriFunc.Call(in) 
		ret := []interface{}{}
		for _,v := range(out) {
			ret = append(ret,v.Interface())
		}
		this.queue.Post(callback,ret...)
	}()
}

func AsynWrap(queue *kendynet.EventQueue,oriFunc interface{}) *caller {

	if nil == queue {
		return nil
	}

	v := reflect.ValueOf(oriFunc)

	if v.Kind() != reflect.Func {
		return nil
	}

	return &caller{
		queue : queue,
		oriFunc : v,
	}
}


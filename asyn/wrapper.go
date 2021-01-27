package asyn

/*
*  将同步接函数调用转换成基于回调的接口
 */

import (
	"github.com/sniperHW/kendynet/util"
	"reflect"
)

type AsynWraper struct {
	fn       interface{}
	callback interface{}
	fnRuner  func(func())
}

func NewAsynWraper(fn interface{}, callback interface{}, fnRuner func(func())) AsynWraper {
	if reflect.ValueOf(fn).Kind() != reflect.Func || reflect.ValueOf(callback).Kind() != reflect.Func {
		panic("invaild fn or callback")
	} else {
		return AsynWraper{
			fn:       fn,
			callback: callback,
			fnRuner:  fnRuner,
		}
	}
}

func (this AsynWraper) Call(args ...interface{}) {

	f := func() {
		out := util.Call(this.fn, args...)
		util.Call(this.callback, out...)
	}

	if nil != this.fnRuner {
		this.fnRuner(f)
	} else {
		go f()
	}
}

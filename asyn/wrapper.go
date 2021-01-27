package asyn

/*
*  将同步接函数调用转换成基于回调的接口
 */

import (
	"github.com/sniperHW/kendynet"
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
		if out, err := util.ProtectCall(this.fn, args...); err != nil {
			logger := kendynet.GetLogger()
			if logger != nil {
				logger.Errorln(err)
			} else {
				panic(err)
			}
		} else if _, err = util.ProtectCall(this.callback, out...); err != nil {
			logger := kendynet.GetLogger()
			if logger != nil {
				logger.Errorln(err)
			} else {
				panic(err)
			}
		}
	}

	if nil != this.fnRuner {
		this.fnRuner(f)
	} else {
		go f()
	}
}

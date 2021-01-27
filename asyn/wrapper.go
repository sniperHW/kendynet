package asyn

/*
*  将同步接函数调用转换成基于回调的接口
 */

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"reflect"
)

type asynfunc func(callback interface{}, args ...interface{})

func AsynWrap(fn interface{}, doCallBack asynfunc, pool ...*routinePool) (wrapFunc asynfunc) {
	if reflect.ValueOf(fn).Kind() == reflect.Func {
		wrapFunc = func(callback interface{}, args ...interface{}) {
			f := func() {
				if out, err := util.ProtectCall(fn, args...); err != nil {
					logger := kendynet.GetLogger()
					if logger != nil {
						logger.Errorln(err)
					} else {
						panic(err)
					}
				} else {
					doCallBack(callback, out...)
				}
			}

			if len(pool) > 0 && nil != pool[0] {
				pool[0].AddTask(f)
			} else {
				go f()
			}
		}
	}
	return
}

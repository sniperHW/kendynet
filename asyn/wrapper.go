package asyn

/*
*  将同步接函数调用转换成基于回调的接口
 */

import (
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/util"
	"reflect"
)

type wrapFunc func(callback interface{}, args ...interface{})

type AsynWraper struct {
	priority     int
	queue        *event.EventQueue
	routinePool_ *routinePool
}

func NewAsynWraper(priority int, queue *event.EventQueue, routinePool_ *routinePool) *AsynWraper {
	if nil == queue {
		return nil
	} else {
		return &AsynWraper{
			priority:     priority,
			queue:        queue,
			routinePool_: routinePool_,
		}
	}
}

func (this *AsynWraper) Wrap(fn interface{}) wrapFunc {
	oriF := reflect.ValueOf(fn)

	if oriF.Kind() != reflect.Func {
		return nil
	}

	return func(callback interface{}, args ...interface{}) {
		f := func() {
			out, err := util.ProtectCall(fn, args...)
			if err != nil {
				logger := kendynet.GetLogger()
				if logger != nil {
					logger.Errorln(err)
				}
				return
			}

			if len(out) > 0 {
				if nil != callback {
					this.queue.PostNoWait(this.priority, callback, out...)
				}
			} else {
				if nil != callback {
					this.queue.PostNoWait(this.priority, callback)
				}
			}
		}

		if nil == this.routinePool_ {
			go f()
		} else {
			//设置了go程池，交给go程池执行
			this.routinePool_.AddTask(f)
		}
	}
}

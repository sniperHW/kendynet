package util

//go test -covermode=count -v -run=.
import (
	"fmt"
	"github.com/stretchr/testify/assert"
	//"reflect"
	"testing"
	"time"
)

type fmtLogger struct {
}

func (this *fmtLogger) Debugf(format string, v ...interface{}) { fmt.Printf(format, v...) }
func (this *fmtLogger) Debug(v ...interface{})                 { fmt.Println(v...) }
func (this *fmtLogger) Infof(format string, v ...interface{})  { fmt.Printf(format, v...) }
func (this *fmtLogger) Info(v ...interface{})                  { fmt.Println(v...) }
func (this *fmtLogger) Warnf(format string, v ...interface{})  { fmt.Printf(format, v...) }
func (this *fmtLogger) Warn(v ...interface{})                  { fmt.Println(v...) }
func (this *fmtLogger) Errorf(format string, v ...interface{}) { fmt.Printf(format, v...) }
func (this *fmtLogger) Error(v ...interface{})                 { fmt.Println(v...) }
func (this *fmtLogger) Fatalf(format string, v ...interface{}) { fmt.Printf(format, v...) }
func (this *fmtLogger) Fatal(v ...interface{})                 { fmt.Println(v...) }
func (this *fmtLogger) SetLevelByString(level string)          {}

func TestPCall(t *testing.T) {
	f1 := func() {
		var ptr *int
		*ptr = 1
	}

	_, err := ProtectCall(f1)
	assert.NotNil(t, err)
	fmt.Println("----------------err--------------\n", err.Error(), "\n----------------err--------------")

	f2 := func(a int) int {
		stack := CallStack(10)
		fmt.Println("----------------stack of f2--------------\n", stack, "\n----------------stack of f2--------------")
		return a + 2
	}

	ret, err := ProtectCall(f2, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, 3, ret[0].(int))

	_, err = ProtectCall(ret)
	assert.Equal(t, ErrArgIsNotFunc, err)

}

func TestRecover(t *testing.T) {
	f1 := func() {
		defer Recover(&fmtLogger{})
		var ptr *int
		*ptr = 1
	}

	f1()
}

func TestFormatFileLine(t *testing.T) {
	fmt.Println(FormatFileLine("%d", 1))
}

func TestNotifyer(t *testing.T) {
	notifyer := NewNotifyer()
	notifyer.Notify()
	notifyer.Notify()
	assert.Nil(t, notifyer.Wait())

	time.AfterFunc(time.Second, func() {
		notifyer.Notify()
	})
	assert.Nil(t, notifyer.Wait())

	close(notifyer.notiChan)

	assert.Equal(t, ErrNotifyerClosed, notifyer.Wait())

}

type Ele struct {
	heapIdx int
	value   int
}

func (this *Ele) Less(o HeapElement) bool {
	return this.value < o.(*Ele).value
}

func (this *Ele) GetIndex() int {
	return this.heapIdx
}

func (this *Ele) SetIndex(idx int) {
	this.heapIdx = idx
}

func TestMinHeap(t *testing.T) {
	heap := NewMinHeap(3)

	ele1 := &Ele{value: 10}
	ele2 := &Ele{value: 20}
	ele3 := &Ele{value: 5}

	heap.Insert(ele1)
	heap.Insert(ele2)
	heap.Insert(ele3)

	assert.Equal(t, 3, heap.Size())
	assert.Equal(t, 5, heap.Min().(*Ele).value)

	assert.Equal(t, 5, heap.PopMin().(*Ele).value)
	assert.Equal(t, 10, heap.PopMin().(*Ele).value)
	assert.Equal(t, 20, heap.PopMin().(*Ele).value)

	heap.Insert(ele1)
	heap.Insert(ele2)
	heap.Insert(ele3)

	ele3.value = 100
	heap.Fix(ele3)

	assert.Equal(t, 10, heap.PopMin().(*Ele).value)
	assert.Equal(t, 20, heap.PopMin().(*Ele).value)
	assert.Equal(t, 100, heap.PopMin().(*Ele).value)

	heap.Insert(ele1)
	heap.Insert(ele2)
	heap.Insert(ele3)

	heap.Remove(ele3)
	assert.Equal(t, -1, ele3.GetIndex())
	assert.Equal(t, 2, heap.Size())
	assert.Equal(t, 10, heap.PopMin().(*Ele).value)
	assert.Equal(t, 20, heap.PopMin().(*Ele).value)

	heap.Insert(ele1)
	heap.Insert(ele2)
	heap.Insert(ele3)
	heap.Clear()
	assert.Equal(t, 0, heap.Size())
	assert.Equal(t, -1, ele1.GetIndex())
	assert.Equal(t, -1, ele2.GetIndex())
	assert.Equal(t, -1, ele3.GetIndex())

	heap.Insert(ele1)
	heap.Insert(ele2)
	heap.Insert(ele3)
	ele4 := &Ele{value: 1}
	heap.Insert(ele4)
	assert.Equal(t, 4, heap.Size())

	assert.Equal(t, 1, heap.PopMin().(*Ele).value)
	assert.Equal(t, 10, heap.PopMin().(*Ele).value)
	assert.Equal(t, 20, heap.PopMin().(*Ele).value)
	assert.Equal(t, 100, heap.PopMin().(*Ele).value)

}

func TestHook(t *testing.T) {

	{

		f1 := func() {
			fmt.Println("i'm f1")
		}

		before := false
		after := false

		Hook1(f1, func() (bool, []interface{}) {
			before = true
			return true, nil
		}, func() {
			after = true
		}).(func())()

		assert.Equal(t, true, before)
		assert.Equal(t, true, after)

	}

	{

		f2 := func(msg string) {
			fmt.Println("i'm f2", msg)
		}

		before := false
		after := false

		Hook1(f2, func() (bool, []interface{}) {
			before = true
			return true, nil
		}, func() {
			after = true
		}).(func(string))("hello")

		assert.Equal(t, true, before)
		assert.Equal(t, true, after)

	}

	{

		f3 := func() string {
			fmt.Println("i'm f3")
			return "f3"
		}

		before := false
		after := false

		ret := Hook1(f3, func() (bool, []interface{}) {
			before = true
			return true, nil
		}, func() {
			after = true
		}).(func() string)()

		assert.Equal(t, true, before)
		assert.Equal(t, true, after)
		assert.Equal(t, "f3", ret)
	}

	{

		f4 := func(msg string) string {
			fmt.Println("i'm f4", msg)
			return msg + "f4"
		}

		before := false
		after := false

		ret := Hook1(f4, func() (bool, []interface{}) {
			before = true
			return true, nil
		}, func() {
			after = true
		}).(func(string) string)("hello")

		assert.Equal(t, true, before)
		assert.Equal(t, true, after)
		assert.Equal(t, "hellof4", ret)
	}

	{

		f5 := func(msg1, msg2 string) {
			fmt.Println("f5", msg1, msg2)
		}

		before := false
		after := false

		Hook1(f5, func() (bool, []interface{}) {
			before = true
			return true, nil
		}, func() {
			after = true
		}).(func(string, string))("hello", "world")

		assert.Equal(t, true, before)
		assert.Equal(t, true, after)
	}

	{

		f6 := func(msgs ...string) {
			fmt.Println("f6", msgs)
		}

		before := false
		after := false

		Hook1(f6, func() (bool, []interface{}) {
			before = true
			return true, nil
		}, func() {
			after = true
		}).(func(...string))("hello", "world")

		assert.Equal(t, true, before)
		assert.Equal(t, true, after)
	}

	{
		f7 := func() string {
			fmt.Println("f7")
			return "f7"
		}

		before := false
		after := false

		ret := Hook1(f7, func() (bool, []interface{}) {
			before = true
			return false, []interface{}{"f7 failed"}
		}, func() {
			after = true
		}).(func() string)()

		assert.Equal(t, true, before)
		assert.Equal(t, false, after)
		assert.Equal(t, ret, "f7 failed")

		before = false
		after = false

		ret = Hook1(f7, func() (bool, []interface{}) {
			before = true
			return true, []interface{}{"f7 failed"}
		}, func() {
			after = true
		}).(func() string)()

		assert.Equal(t, true, before)
		assert.Equal(t, true, after)
		assert.Equal(t, ret, "f7")

	}

	{

		f := func(v string) {
			fmt.Println("f", v)
		}

		Hook2(f, func() {
			fmt.Println("before f")
		}, func() {
			fmt.Println("after f")
		}).(func(string))("hello")

		Hook2(f, func(v string) {
			fmt.Println("before f", v)
		}, func(v string) {
			fmt.Println("after f", v)
		}).(func(string))("hello")

	}

	{

		f := func(v int) int {
			fmt.Println("v", v)
			return v + 1
		}

		ret := Hook3(f, func(v int) int {
			fmt.Println("v", v)
			return v + 1
		}, func(v int) int {
			fmt.Println("v", v)
			return v + 1
		}).(func(int) int)(1)

		fmt.Println(ret)

	}

}

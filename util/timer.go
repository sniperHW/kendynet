package util

/*
import( 
	"time"
	"sync"
	"github.com/sniperHW/kendynet"
)

type timer struct {
	expired    time.Time               //到期时间
	eventQue  *kendynet.EventQueue     
	timeout    time.Duration
	callback   func()
}


var	(
	wakeup chan bool
	minheap *MinHeap
	pool sync.Pool 
)


func get() *timer {
	return pool.Get().(*timer)
}

func put(t *timer) {
	pool.Put(t)
}

func pcall(callback func()) {
	defer func(){
		if r := recover(); r != nil {
			buf := make([]byte, 65535)
			l := runtime.Stack(buf, false)
			err := fmt.Errorf("%v: %s", r, buf[:l])
			Errorf(util.FormatFileLine("%s\n",err.Error()))
		}			
	}()
	callback()
}

func loop() {

	defaultSleepTime := 10 * time.Second
	//ticker := time.NewTicker(defaultSleepTime)
	for {
		now := time.Now()
		
		for {
			min := minheap.Min()
			if nil != min && min.(*timer).expired.After(now) {
				t := min.(*timer)
				if nil != t.eventQue {

				} else {

				}
				minheap.PopMin()
			}		
		}
	}
}

func init() {
	wakeup  = make(chan bool,65536)
	minheap = NewMinHeap(65536)
	pool    = sync.Pool{
		New : func interface{} {
			return &timer{}
		},
	}
	go loop()
}


*/
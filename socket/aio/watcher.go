// +build darwin netbsd freebsd openbsd dragonfly linux

package aio

import (
	//"fmt"
	"github.com/sniperHW/aiogo"
	"math/rand"
	"runtime"
)

var watchers []*aiogo.Watcher
var readCompleteQueues []*aiogo.CompleteQueue
var writeCompleteQueue []*aiogo.CompleteQueue

func completeRoutine(completeQueue *aiogo.CompleteQueue) {
	for {
		es, ok := completeQueue.Get()
		if !ok {
			return
		} else {
			c := es.Ud.(*AioSocket)
			if es.Type == aiogo.User {
				//fmt.Println("process aiogo.User")
				c.postSend()
			} else if es.Type == aiogo.Read {
				c.onRecvComplete(es)
			} else {
				c.onSendComplete(es)
			}
		}
	}
}

func getWatcherAndCompleteQueue() (*aiogo.Watcher, *aiogo.CompleteQueue, *aiogo.CompleteQueue) {
	r := rand.Int()
	return watchers[r%len(watchers)], readCompleteQueues[r%len(readCompleteQueues)], writeCompleteQueue[r%len(writeCompleteQueue)]
}

func Init(watcherCount int, completeQueueCount int, bufferPool ...aiogo.BufferPool) error {
	if watcherCount <= 0 {
		watcherCount = 1
	}

	if completeQueueCount <= 0 {
		completeQueueCount = runtime.NumCPU() * 2
	}

	for i := 0; i < watcherCount; i++ {
		watcher, err := aiogo.NewWatcher(runtime.NumCPU(), bufferPool...)
		if nil != err {
			return err
		}
		watchers = append(watchers, watcher)
	}

	for i := 0; i < completeQueueCount; i++ {
		queue := aiogo.NewCompleteQueueWithSpinlock()
		readCompleteQueues = append(readCompleteQueues, queue)
		go completeRoutine(queue)
	}

	for i := 0; i < completeQueueCount; i++ {
		queue := aiogo.NewCompleteQueueWithSpinlock()
		writeCompleteQueue = append(writeCompleteQueue, queue)
		go completeRoutine(queue)
	}

	return nil
}

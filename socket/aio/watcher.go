// +build darwin netbsd freebsd openbsd dragonfly linux

package aio

import (
	"github.com/sniperHW/aiogo"
	"math/rand"
	"runtime"
)

var watchers []*aiogo.Watcher
var readCompleteQueues []*aiogo.CompleteQueue
var writeCompleteQueues []*aiogo.CompleteQueue

func completeRoutine(completeQueue *aiogo.CompleteQueue) {
	for {
		es, ok := completeQueue.Get()
		if !ok {
			return
		} else {
			//if es.Type == aiogo.User {
			//	es.Ud.(func())()
			//} else {
			c := es.Ud.(*AioSocket)
			if es.Type == aiogo.Read {
				c.onRecvComplete(es)
			} else {
				c.onSendComplete(es)
			}
			//}
		}
	}
}

func getWatcherAndCompleteQueue() (*aiogo.Watcher, *aiogo.CompleteQueue, *aiogo.CompleteQueue) {
	r := rand.Int()
	return watchers[r%len(watchers)], readCompleteQueues[r%len(readCompleteQueues)], writeCompleteQueues[r%len(writeCompleteQueues)]
}

func Init(watcherCount int, completeQueueCount int, workerCount int, buffPool aiogo.BufferPool) error {
	if watcherCount <= 0 {
		watcherCount = 1
	}

	if completeQueueCount <= 0 {
		completeQueueCount = runtime.NumCPU() * 2
	}

	for i := 0; i < watcherCount; i++ {
		watcher, err := aiogo.NewWatcher(&aiogo.WatcherOption{
			BufferPool:     buffPool,
			NotifyOnlyMode: true,
			WorkerCount:    workerCount,
		})
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
		writeCompleteQueues = append(writeCompleteQueues, queue)
		go completeRoutine(queue)
	}

	return nil
}

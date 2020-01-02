// +build darwin netbsd freebsd openbsd dragonfly linux

package aio

import (
	"github.com/sniperHW/aiogo"
	"math/rand"
	"runtime"
)

var watchers []*aiogo.Watcher
var completeQueues []*aiogo.CompleteQueue

func completeRoutine(completeQueue *aiogo.CompleteQueue) {
	for {
		es, ok := completeQueue.Get()
		if !ok {
			return
		} else {
			c := es.Ud.(*AioSocket)
			if es.Type == aiogo.User {
				c.postSend()
			} else if es.Type == aiogo.Read {
				c.onRecvComplete(es)
			} else {
				c.onSendComplete(es)
			}
		}
	}
}

func GetWatcherAndCompleteQueue() (*aiogo.Watcher, *aiogo.CompleteQueue) {
	r := rand.Int()
	return watchers[r%len(watchers)], completeQueues[r%len(completeQueues)]
}

func Init(watcherCount int, completeQueueCount int, bufferPool ...aiogo.BufferPool) error {
	if watcherCount <= 0 {
		watcherCount = 1
	}

	if completeQueueCount <= 0 {
		completeQueueCount = runtime.NumCPU() * 2
	}

	for i := 0; i < watcherCount; i++ {
		watcher, err := aiogo.NewWatcher(bufferPool...)
		if nil != err {
			return err
		}
		watchers = append(watchers, watcher)
	}

	for i := 0; i < completeQueueCount; i++ {
		queue := aiogo.NewCompleteQueueWithSpinlock()
		completeQueues = append(completeQueues, queue)
		go completeRoutine(queue)
	}

	return nil
}

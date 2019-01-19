package rpc

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/util"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var ErrCallTimeout error = fmt.Errorf("rpc call timeout")
var ErrSocketClose error = fmt.Errorf("socket close")

type RPCResponseHandler func(interface{}, error)

var (
	queue       *event.EventQueue
	clients     map[*RPCClient]bool
	client_once sync.Once
	mtx         sync.Mutex
)

type reqContext struct {
	heapIdx    uint32
	seq        uint64
	onResponse RPCResponseHandler
	deadline   time.Time
	timestamp  int64
}

func (this *reqContext) Less(o util.HeapElement) bool {
	return o.(*reqContext).deadline.After(this.deadline)
}

func (this *reqContext) GetIndex() uint32 {
	return this.heapIdx
}

func (this *reqContext) SetIndex(idx uint32) {
	this.heapIdx = idx
}

func (this *reqContext) callResponseCB(ret interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 65535)
			l := runtime.Stack(buf, false)
			err := fmt.Errorf("%v: %s", r, buf[:l])
			kendynet.Errorf(util.FormatFileLine("%s\n", err.Error()))
		}
	}()
	this.onResponse(ret, err)
}

type RPCClient struct {
	encoder  RPCMessageEncoder
	decoder  RPCMessageDecoder
	sequence uint64
	channel  RPCChannel
	minheap  *util.MinHeap
	waitResp map[uint64]*reqContext
}

//通道关闭后调用
func (this *RPCClient) OnChannelClose(err error) {
	mtx.Lock()
	delete(clients, this)
	mtx.Unlock()
	queue.PostNoWait(func() {
		for _, ctx := range this.waitResp {
			ctx.callResponseCB(nil, err)
		}
	})
}

func (this *RPCClient) checkTimeout() {
	now := time.Now()
	for {
		r := this.minheap.Min()
		if r != nil && now.After(r.(*reqContext).deadline) {
			this.minheap.PopMin()
			if _, ok := this.waitResp[r.(*reqContext).seq]; !ok {
				kendynet.Infof("timeout context:%d not found\n", r.(*reqContext).seq)
			} else {
				delete(this.waitResp, r.(*reqContext).seq)
				kendynet.Infof("timeout context:%d\n", r.(*reqContext).seq)
				r.(*reqContext).callResponseCB(nil, ErrCallTimeout)
			}
		} else {
			break
		}
	}
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	msg, err := this.decoder.Decode(message)
	if nil != err {
		kendynet.Errorf(util.FormatFileLine("RPCClient rpc message from(%s) decode err:%s\n", this.channel.Name, err.Error()))
		return
	}

	queue.PostNoWait(func() {
		if context, ok := this.waitResp[msg.GetSeq()]; ok {
			delete(this.waitResp, msg.GetSeq())
			this.minheap.Remove(context)
			resp := msg.(*RPCResponse)
			context.callResponseCB(resp.Ret, resp.Err)
		} else {
			kendynet.Debugf("on response,but missing reqContext:%d\n", msg.GetSeq())
		}
	})
}

//投递，不关心响应和是否失败
func (this *RPCClient) Post(method string, arg interface{}) error {

	req := &RPCRequest{
		Method:   method,
		Seq:      atomic.AddUint64(&this.sequence, 1),
		Arg:      arg,
		NeedResp: false,
	}

	request, err := this.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("encode error:%s\n", err.Error())
	}

	err = this.channel.SendRequest(request)
	if nil != err {
		return ErrSocketClose
	}
	return nil
}

/*
 *  异步调用
 */
func (this *RPCClient) AsynCall(method string, arg interface{}, timeout uint32, cb RPCResponseHandler) {

	if cb == nil {
		panic("cb == nil")
	}

	queue.PostNoWait(func() {
		req := &RPCRequest{
			Method:   method,
			Seq:      atomic.AddUint64(&this.sequence, 1),
			Arg:      arg,
			NeedResp: true,
		}

		request, err := this.encoder.Encode(req)
		if err != nil {
			cb(nil, fmt.Errorf("encode error:%s\n", err.Error()))
			return
		}

		if timeout <= 0 {
			timeout = 5000
		}

		context := &reqContext{}
		context.heapIdx = 0
		context.seq = req.Seq
		context.onResponse = cb
		context.deadline = time.Now().Add(time.Duration(timeout) * time.Millisecond)
		context.timestamp = time.Now().UnixNano()

		err = this.channel.SendRequest(request)
		if nil != err {
			context.callResponseCB(nil, ErrSocketClose)
		} else {
			this.waitResp[context.seq] = context
			this.minheap.Insert(context)
		}
	})
}

//同步调用
//同步调用
func (this *RPCClient) SyncCall(method string, arg interface{}, timeout uint32) (ret interface{}, err error) {
	respChan := make(chan interface{})
	f := func(ret_ interface{}, err_ error) {
		ret = ret_
		err = err_
		respChan <- nil
	}
	this.AsynCall(method, arg, timeout, f)
	_ = <-respChan
	return
}

func NewClient(channel RPCChannel, decoder RPCMessageDecoder, encoder RPCMessageEncoder) *RPCClient {
	if nil == decoder {
		panic("decoder == nil")
	}

	if nil == encoder {
		panic("encoder == nil")
	}

	if nil == channel {
		panic("channel == nil")
	}

	r := &RPCClient{
		encoder:  encoder,
		decoder:  decoder,
		channel:  channel,
		minheap:  util.NewMinHeap(1024),
		waitResp: map[uint64]*reqContext{},
	}
	mtx.Lock()
	clients[r] = true
	mtx.Unlock()
	return r
}

func InitClient(queue_ *event.EventQueue) {
	client_once.Do(func() {
		clients = map[*RPCClient]bool{}

		if nil != queue_ {
			queue = queue_
		} else {
			queue = event.NewEventQueue()
			go func() {
				queue.Run()
			}()
		}

		//启动一个go程，每10毫秒向queue投递一个定时器消息
		go func() {
			for {
				time.Sleep(time.Duration(10) * time.Millisecond)
				queue.PostNoWait(func() {
					c := []*RPCClient{}
					mtx.Lock()
					for r, _ := range clients {
						c = append(c, r)
					}
					mtx.Unlock()
					for _, r := range c {
						r.checkTimeout()
					}
				})
			}
		}()
	})
}

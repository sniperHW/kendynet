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

var (
	client_once sync.Once
	clients     map[*RPCClient]bool
	mtx         sync.Mutex
)

type RPCResponseHandler func(interface{}, error)

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
			kendynet.Errorf(util.FormatFileLine("%s\n", fmt.Sprintf("%v: %s", r, buf[:l])))
		}
	}()
	this.onResponse(ret, err)
}

type RPCClient struct {
	encoder      RPCMessageEncoder
	decoder      RPCMessageDecoder
	sequence     uint64
	channel      RPCChannel
	mtx          sync.Mutex
	minheap      *util.MinHeap
	waitResp     map[uint64]*reqContext //待响应的请求
	cbEventQueue *event.EventQueue
}

func (this *RPCClient) callResponseCB(ctx *reqContext, ret interface{}, err error) {
	if this.cbEventQueue != nil {
		this.cbEventQueue.PostNoWait(func() {
			ctx.callResponseCB(ret, err)
		})
	} else {
		ctx.callResponseCB(ret, err)
	}
}

func (this *RPCClient) checkTimeout() {
	now := time.Now()
	timeout := []*reqContext{}
	this.mtx.Lock()
	for {
		r := this.minheap.Min()
		if r != nil && now.After(r.(*reqContext).deadline) {
			this.minheap.PopMin()
			if _, ok := this.waitResp[r.(*reqContext).seq]; !ok {
				kendynet.Infof("timeout context:%d not found\n", r.(*reqContext).seq)
			} else {
				delete(this.waitResp, r.(*reqContext).seq)
				timeout = append(timeout, r.(*reqContext))
				kendynet.Infof("timeout context:%d\n", r.(*reqContext).seq)
			}
		} else {
			break
		}
	}
	this.mtx.Unlock()

	for _, v := range timeout {
		this.callResponseCB(v, nil, ErrCallTimeout)
	}
}

//通道关闭后调用
func (this *RPCClient) OnChannelClose(err error) {

	mtx.Lock()
	delete(clients, this)
	mtx.Unlock()

	var waitRespBack map[uint64]*reqContext

	this.mtx.Lock()
	waitRespBack = this.waitResp
	this.waitResp = map[uint64]*reqContext{}
	this.minheap.Clear()
	this.mtx.Unlock()

	for _, v := range waitRespBack {
		this.callResponseCB(v, nil, err)
	}
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	msg, err := this.decoder.Decode(message)
	if nil != err {
		kendynet.Errorf(util.FormatFileLine("RPCClient rpc message from(%s) decode err:%s\n", this.channel.Name, err.Error()))
		return
	}

	var (
		ctx *reqContext
		ok  bool
	)

	this.mtx.Lock()
	if ctx, ok = this.waitResp[msg.GetSeq()]; ok {
		delete(this.waitResp, msg.GetSeq())
		this.minheap.Remove(ctx)
	}
	this.mtx.Unlock()

	if nil != ctx {
		resp := msg.(*RPCResponse)
		this.callResponseCB(ctx, resp.Ret, resp.Err)
	} else {
		kendynet.Debugf("on response,but missing reqContext:%d\n", msg.GetSeq())
	}
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
		return err
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

	req := &RPCRequest{
		Method:   method,
		Seq:      atomic.AddUint64(&this.sequence, 1),
		Arg:      arg,
		NeedResp: true,
	}

	if timeout <= 0 {
		timeout = 5000
	}

	context := &reqContext{
		onResponse: cb,
		seq:        req.Seq,
	}

	request, err := this.encoder.Encode(req)
	if err != nil {
		this.callResponseCB(context, nil, fmt.Errorf("encode error:%s\n", err.Error()))
	} else {

		context.deadline = time.Now().Add(time.Duration(timeout) * time.Millisecond)
		context.timestamp = time.Now().UnixNano()

		this.mtx.Lock()
		this.waitResp[context.seq] = context
		this.minheap.Insert(context)
		this.mtx.Unlock()

		err := this.channel.SendRequest(request)

		if nil != err {
			this.mtx.Lock()
			delete(this.waitResp, context.seq)
			this.minheap.Remove(context)
			this.mtx.Unlock()
			if this.cbEventQueue != nil {
				this.cbEventQueue.PostNoWait(func() {
					context.callResponseCB(nil, err)
				})
			} else {
				context.callResponseCB(nil, err)
			}
		}
	}
}

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

func onceRoutine(r *RPCClient) {

	client_once.Do(func() {
		clients = map[*RPCClient]bool{}
		go func() {
			c := []*RPCClient{}
			for {
				time.Sleep(time.Duration(10) * time.Millisecond)
				mtx.Lock()
				for r, _ := range clients {
					c = append(c, r)
				}
				mtx.Unlock()
				for _, r := range c {
					r.checkTimeout()
				}

				c = c[0:0]
			}
		}()
	})

	mtx.Lock()
	clients[r] = true
	mtx.Unlock()
}

func NewClient(channel RPCChannel, decoder RPCMessageDecoder, encoder RPCMessageEncoder, cbEventQueue ...*event.EventQueue) (*RPCClient, error) {
	if nil == decoder {
		return nil, fmt.Errorf("decoder == nil")
	}

	if nil == encoder {
		return nil, fmt.Errorf("encoder == nil")
	}

	if nil == channel {
		return nil, fmt.Errorf("channel == nil")
	}

	var q *event.EventQueue

	if len(cbEventQueue) > 0 {
		q = cbEventQueue[0]
	}

	r := &RPCClient{
		encoder:      encoder,
		decoder:      decoder,
		channel:      channel,
		cbEventQueue: q,
		minheap:      util.NewMinHeap(1024),
		waitResp:     map[uint64]*reqContext{},
	}

	onceRoutine(r)

	return r, nil
}

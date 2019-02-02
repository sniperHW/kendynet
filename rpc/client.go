package rpc

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/util"
	"sync"
	"sync/atomic"
	"time"
)

var ErrCallTimeout error = fmt.Errorf("rpc call timeout")

var client_once sync.Once
var sequence uint64
var mtx sync.Mutex
var minheap *util.MinHeap = util.NewMinHeap(4096)
var waitResp map[uint64]*reqContext = map[uint64]*reqContext{} //待响应的请求

type RPCResponseHandler func(interface{}, error)

type reqContext struct {
	heapIdx      uint32
	seq          uint64
	onResponse   RPCResponseHandler
	deadline     time.Time
	cbEventQueue *event.EventQueue
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
	if this.cbEventQueue != nil {
		this.cbEventQueue.PostNoWait(func() {
			this.callResponseCB_(ret, err)
		})
	} else {
		this.callResponseCB_(ret, err)
	}
}

func (this *reqContext) callResponseCB_(ret interface{}, err error) {
	defer util.Recover(kendynet.GetLogger())
	this.onResponse(ret, err)
}

type RPCClient struct {
	encoder      RPCMessageEncoder
	decoder      RPCMessageDecoder
	cbEventQueue *event.EventQueue
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	msg, err := this.decoder.Decode(message)
	if nil != err {
		kendynet.Errorf(util.FormatFileLine("RPCClient rpc message decode err:%s\n", err.Error()))
		return
	}

	switch msg.(type) {
	case *RPCResponse:
		break
	default:
		panic("RPCClient.OnRPCMessage() invaild msg type")
	}

	var (
		ctx *reqContext
		ok  bool
	)

	mtx.Lock()
	if ctx, ok = waitResp[msg.GetSeq()]; ok {
		delete(waitResp, msg.GetSeq())
		minheap.Remove(ctx)
	}
	mtx.Unlock()

	if nil != ctx {
		resp := msg.(*RPCResponse)
		ctx.callResponseCB(resp.Ret, resp.Err)
	} else {
		kendynet.Debugf("on response,but missing reqContext:%d\n", msg.GetSeq())
	}
}

//投递，不关心响应和是否失败
func (this *RPCClient) Post(channel RPCChannel, method string, arg interface{}) error {

	req := &RPCRequest{
		Method:   method,
		Seq:      atomic.AddUint64(&sequence, 1),
		Arg:      arg,
		NeedResp: false,
	}

	request, err := this.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("encode error:%s\n", err.Error())
	}

	err = channel.SendRequest(request)
	if nil != err {
		return err
	}
	return nil
}

/*
 *  异步调用
 */

func (this *RPCClient) AsynCall(channel RPCChannel, method string, arg interface{}, timeout time.Duration, cb RPCResponseHandler) error {

	if cb == nil {
		panic("cb == nil")
	}

	req := &RPCRequest{
		Method:   method,
		Seq:      atomic.AddUint64(&sequence, 1),
		Arg:      arg,
		NeedResp: true,
	}

	context := &reqContext{
		onResponse:   cb,
		seq:          req.Seq,
		cbEventQueue: this.cbEventQueue,
	}

	request, err := this.encoder.Encode(req)
	if err != nil {
		return err
	} else {
		context.deadline = time.Now().Add(timeout)
		mtx.Lock()
		defer mtx.Unlock()
		err := channel.SendRequest(request)
		if err == nil {
			waitResp[context.seq] = context
			minheap.Insert(context)
			return nil
		} else {
			return err
		}
	}
}

//同步调用
func (this *RPCClient) Call(channel RPCChannel, method string, arg interface{}, timeout time.Duration) (ret interface{}, err error) {
	respChan := make(chan interface{})
	f := func(ret_ interface{}, err_ error) {
		ret = ret_
		err = err_
		respChan <- nil
	}
	err = this.AsynCall(channel, method, arg, timeout, f)
	if nil == err {
		_ = <-respChan
	}
	return
}

func onceRoutine() {

	client_once.Do(func() {
		go func() {
			for {
				time.Sleep(time.Duration(10) * time.Millisecond)
				timeout := []*reqContext{}
				mtx.Lock()
				for {
					now := time.Now()
					r := minheap.Min()
					if r != nil && now.After(r.(*reqContext).deadline) {
						minheap.PopMin()
						if _, ok := waitResp[r.(*reqContext).seq]; !ok {
							kendynet.Infof("timeout context:%d not found\n", r.(*reqContext).seq)
						} else {
							delete(waitResp, r.(*reqContext).seq)
							timeout = append(timeout, r.(*reqContext))
							kendynet.Infof("timeout context:%d\n", r.(*reqContext).seq)
						}
					} else {
						break
					}
				}
				mtx.Unlock()

				for _, v := range timeout {
					v.callResponseCB(nil, ErrCallTimeout)
				}
			}
		}()
	})
}

func NewClient(decoder RPCMessageDecoder, encoder RPCMessageEncoder, cbEventQueue ...*event.EventQueue) *RPCClient {
	if nil == decoder {
		panic("decoder == nil")
	}

	if nil == encoder {
		panic("encoder == nil")
	}

	var q *event.EventQueue

	if len(cbEventQueue) > 0 {
		q = cbEventQueue[0]
	}

	onceRoutine()

	return &RPCClient{
		encoder:      encoder,
		decoder:      decoder,
		cbEventQueue: q,
	}
}

package rpc

import (
	"sync"
	"fmt"
	"sync/atomic"
	"time"
	"github.com/sniperHW/kendynet/util"
)

type RPCResponseHandler func(interface{},error)

type reqContext struct {
	heapIdx     uint32
	seq         uint64
	onResponse  RPCResponseHandler
	deadline    time.Time
}

func (this *reqContext) Less(o util.HeapElement) bool {
	other := o.(*reqContext)
	return other.deadline.After(this.deadline)
}

func (this *reqContext) GetIndex() uint32 {
	return this.heapIdx
}

func (this *reqContext) SetIndex(idx uint32) {
	this.heapIdx = idx
}

const (
	tt_request = 1
	tt_channel_close = 2
	tt_response = 3
	tt_timer    = 4
)

type message struct {
	tt int32
	data1 interface{}
	data2 interface{}
}

type channelContext struct {
	minheap          *util.MinHeap
	pendingCalls      map[uint64]*reqContext
}

type reqContextMgr struct {
	queue            *util.BlockQueue   
	channels         map[RPCChannel]*channelContext
}

func (this *reqContextMgr) pushRequest(channel *RPCClient,req *reqContext) {
	m := &message{tt:tt_request,data1:channel,data2:req}
	this.queue.Add(m)
}

func (this *reqContextMgr) pushClose(channel *RPCClient,err error) {
	m := &message{tt:tt_channel_close,data1:channel,data2:err}
	this.queue.Add(m)
}

func (this *reqContextMgr) pushResponse(channel *RPCClient,rpcMsg RPCMessage) {
	m := &message{tt:tt_response,data1:channel,data2:rpcMsg}
	this.queue.Add(m)
}

func (this *reqContextMgr) pushTimer() {
	m := &message{tt:tt_timer}
	this.queue.Add(m)
}

func (this *reqContextMgr) onRequest(msg message) {
	var ok bool
	var cContext *channelContext
	channel := msg.data1.(RPCChannel)
	context := msg.data2.(*reqContext)
	if ok,cContext = this.channels[channel] ; !ok {
		cContext = &channelContext{minheap:NewMinHeap(1024),pendingCalls:map[uint64]*reqContext{}}
		this.channels[channel] = cContext
	}
	cContext.pendingCalls[context.Seq] = context
	if context.deadline != time.Duration(0) {
		cContext.minheap.Insert(context)
	}
}

func (this *reqContextMgr) onClose(msg message) {
	channel := msg.data1.(RPCChannel)
	err     := msg.data2.(error)
	if ok,cContext := this.channels[channel] ; ok {
		delete(this.channels,channel)
		for _ , v := range cContext.pendingCalls {
			v.onResponse(nil,err)
		}
	}	
}

func (this *reqContextMgr) onReponse(msg message) {
	var ok bool
	var cContext *channelContext
	var context  *reqContext
	channel := msg.data1.(RPCChannel)
	rpcMsg  := msg.data2.(RPCMessage)
	if ok,cContext = this.channels[channel] ; ok {
		if ok,context = cContext.pendingCalls[rpcMsg.GetSeq()] ; ok {
			delete(cContext.pendingCalls,rpcMsg.GetSeq())
			cContext.minheap.Remove(context)
			resp := rpcMsg.(*RPCResponse)
			context.onResponse(resp.Ret,resp.Err)			
		}
	}	
}

func (this *reqContextMgr) loop() {
	for {
		_,localList := this.queue.Get()
		size := len(localList)
		for i := 0; i < size; i++ {
			msg := localList[i].(message)
			switch(msg.tt) {
				case tt_request:
					this.onRequest(msg)
					break
				case tt_channel_close:
					this.onClose(msg)
					break
				case tt_response:
					this.onResponse(msg)
					break
				default:
					break	
			}
		}
	}
}

/*
type reqContext struct {
	heapIdx     uint32
	seq         uint64
	onResponse  RPCResponseHandler
	deadline    time.Time
}

func (this *reqContext) Less(o util.HeapElement) bool {
	other := o.(*reqContext)
	return other.deadline.After(this.deadline)
}

func (this *reqContext) GetIndex() uint32 {
	return this.heapIdx
}

func (this *reqContext) SetIndex(idx uint32) {
	this.heapIdx = idx
}


type RPCClient struct {
	encoder   		  	RPCMessageEncoder
	decoder   		  	RPCMessageDecoder
	sequence            uint64
	mutex             	sync.Mutex
	channel             RPCChannel             
}

//通道关闭后调用
func (this *RPCClient) OnChannelClose(err error) {
	this.mutex.Lock()
	this.channel = nil
	this.minheap.Clear()
	pendingCalls := this.pendingCalls
	this.pendingCalls = nil
	this.mutex.Unlock()
	for _ , v := range pendingCalls {
		v.onResponse(nil,err)
	}
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	msg,err := this.decoder.Decode(message)
	if nil != err {
		Errorf(util.FormatFileLine("RPCClient rpc message from(%s) decode err:%s\n",this.channel.Name,err.Error()))
		return
	}
	this.mutex.Lock()
	pendingReq,ok := this.pendingCalls[msg.GetSeq()] 
	if !ok {
		this.mutex.Unlock()
		return
	}
	delete(this.pendingCalls,msg.GetSeq())
	this.minheap.Remove(pendingReq)
	this.mutex.Unlock()
	resp := msg.(*RPCResponse)
	pendingReq.onResponse(resp.Ret,resp.Err)
}

//投递，不关心响应和是否失败
func (this *RPCClient) Post(method string,arg interface{}) error {
	var channel RPCChannel
	this.mutex.Lock()
	channel = this.channel
	this.mutex.Unlock()

	if nil == channel {
		return fmt.Errorf("channel closed")
	}

	req := &RPCRequest{} 
	req.Method = method
	req.Seq = atomic.AddUint64(&this.sequence,1) 
	req.Arg = arg
	req.NeedResp = false

	request,err := this.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("encode error:%s\n",err.Error())
	} 

	err = channel.SendRequest(request)
	if nil != err {
		return err
	}
	return nil
}

//异步调用
func (this *RPCClient) AsynCall(method string,arg interface{},timeout uint32,cb RPCResponseHandler) error {

	if cb == nil {
		return fmt.Errorf("cb == nil")
	}

	var channel RPCChannel
	this.mutex.Lock()
	channel = this.channel
	this.mutex.Unlock()

	if nil == channel {	
		return fmt.Errorf("channel closed")
	}

	req := &RPCRequest{} 
	req.Method = method
	req.Seq = atomic.AddUint64(&this.sequence,1) 
	req.Arg = arg
	req.NeedResp = true

	request,err := this.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("encode error:%s\n",err.Error())
	} 

	err = channel.SendRequest(request)
	if nil != err {
		return err
	}

	this.mutex.Lock()
	if nil != this.pendingCalls {
		r := &reqContext{seq:req.Seq,onResponse:cb}
		this.pendingCalls[req.Seq] = r
		if timeout > 0 {
			r.deadline = time.Now().Add(time.Duration(timeout))
			this.minheap.Insert(r)
			if !this.timeoutRoutine {
				this.timeoutRoutine = true
				this.startRoutine()
			}
		}
	}
	this.mutex.Unlock()
	return nil
}

//同步调用
func (this *RPCClient) SyncCall(method string,arg interface{},timeout uint32) (interface{},error) {
	respChan := make(chan int)

	var result interface{}
	var respError error 

	err := this.AsynCall(method,arg,timeout,func (ret interface{},err error){
		result = ret
		respError = err
		respChan <- 1
	})

	if nil != err {
		return nil,err
	}

	_ = <- respChan

	return result,respError
}


var ErrCallTimeout error = fmt.Errorf("rpc call timeout")

//启动一个goroutine检测call超时
func (this *RPCClient) startRoutine() {
	go func() {
		defer func(){
			this.mutex.Unlock()
		}()
		this.mutex.Lock()
		for {
			now := time.Now()
			for {
				r := this.minheap.Min()
				if nil == r || !now.After(r.(*reqContext).deadline) {
					break
				}
				this.minheap.PopMin()
				delete(this.pendingCalls,r.(*reqContext).seq)
				go func() {
					r.(*reqContext).onResponse(nil,ErrCallTimeout)
				}()
 			}

 			if this.channel == nil {
 				return
 			}

 			sleepTime := int64(10 * time.Millisecond)

 			if r := this.minheap.Min(); r != nil {
 				delta := r.(*reqContext).deadline.UnixNano() - now.UnixNano()
 				if delta < sleepTime {
 					fmt.Println(delta)
 					sleepTime = delta
 				}
 			}
 			this.mutex.Unlock()
 			time.Sleep(time.Duration(sleepTime))
 			this.mutex.Lock()
		}		
	}()
}

func NewRPCClient(channel RPCChannel,decoder RPCMessageDecoder,encoder RPCMessageEncoder) (*RPCClient,error) {
	if nil == decoder {
		return nil,fmt.Errorf("decoder == nil")
	}

	if nil == encoder {
		return nil,fmt.Errorf("encoder == nil")
	}

	if nil == channel {
		return nil,fmt.Errorf("channel == nil")
	}

	c := &RPCClient{}
	c.encoder = encoder
	c.decoder = decoder
	c.pendingCalls = map[uint64]*reqContext{}
	c.minheap = util.NewMinHeap(1024)
	c.channel = channel
	return c,nil
}
*/
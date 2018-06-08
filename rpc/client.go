package rpc

import (
	"fmt"
	"sync/atomic"
	"time"
	"github.com/sniperHW/kendynet/util"
)

var ErrCallTimeout error = fmt.Errorf("rpc call timeout")

type RPCResponseHandler func(interface{},error)

type reqContext struct {
	heapIdx     uint32
	seq         uint64
	onResponse  RPCResponseHandler
	deadline    time.Time
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

func (this *reqContextMgr) pushRequest(channel RPCChannel,req *reqContext) {
	this.queue.Add(&message{tt:tt_request,data1:channel,data2:req})
}

func (this *reqContextMgr) pushClose(channel RPCChannel,err error) {
	this.queue.Add(&message{tt:tt_channel_close,data1:channel,data2:err})
}

func (this *reqContextMgr) pushResponse(channel RPCChannel,rpcMsg RPCMessage) {
	this.queue.Add(&message{tt:tt_response,data1:channel,data2:rpcMsg})
}

func (this *reqContextMgr) pushTimer() {
	this.queue.Add(&message{tt:tt_timer})
}

func (this *reqContextMgr) onRequest(msg *message) {
	var ok bool
	var cContext *channelContext
	channel := msg.data1.(RPCChannel)
	context := msg.data2.(*reqContext)
	if cContext,ok = this.channels[channel] ; !ok {
		cContext = &channelContext{minheap:util.NewMinHeap(1024),pendingCalls:map[uint64]*reqContext{}}
		this.channels[channel] = cContext
	}
	cContext.pendingCalls[context.seq] = context
	if !context.deadline.IsZero() {
		cContext.minheap.Insert(context)
	}
}

func (this *reqContextMgr) onClose(msg *message) {
	channel := msg.data1.(RPCChannel)
	err     := msg.data2.(error)
	if cContext,ok := this.channels[channel] ; ok {
		delete(this.channels,channel)
		for _ , v := range cContext.pendingCalls {
			v.onResponse(nil,err)
		}
	}	
}

func (this *reqContextMgr) onResponse(msg *message) {
	var ok bool
	var cContext *channelContext
	var context  *reqContext
	channel := msg.data1.(RPCChannel)
	rpcMsg  := msg.data2.(RPCMessage)
	if cContext,ok = this.channels[channel] ; ok {
		if context,ok = cContext.pendingCalls[rpcMsg.GetSeq()] ; ok {
			delete(cContext.pendingCalls,rpcMsg.GetSeq())
			cContext.minheap.Remove(context)
			resp := rpcMsg.(*RPCResponse)
			context.onResponse(resp.Ret,resp.Err)			
		}
	}	
}

func (this *reqContextMgr) checkTimeout() {
	now := time.Now()
	for _,v := range this.channels {
		for {
			r := v.minheap.Min()
			if r != nil && now.After(r.(*reqContext).deadline) {
				v.minheap.PopMin()
				delete(v.pendingCalls,r.(*reqContext).seq)
				r.(*reqContext).onResponse(nil,ErrCallTimeout)
			} else {
				break
			}
		}
	}	
}

func (this *reqContextMgr) loop() {
	for {
		_,localList := this.queue.Get()
		for _ , v := range localList {
			msg := v.(*message)
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
		//检查是否有超时回调
		this.checkTimeout()
	}
}

var reqMgr *reqContextMgr

type RPCClient struct {
	encoder   		  	RPCMessageEncoder
	decoder   		  	RPCMessageDecoder
	sequence            uint64
	channel             RPCChannel             
}

//通道关闭后调用
func (this *RPCClient) OnChannelClose(err error) {
	reqMgr.pushClose(this.channel,err)
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	msg,err := this.decoder.Decode(message)
	if nil != err {
		Errorf(util.FormatFileLine("RPCClient rpc message from(%s) decode err:%s\n",this.channel.Name,err.Error()))
		return
	}
	reqMgr.pushResponse(this.channel,msg)
}

//投递，不关心响应和是否失败
func (this *RPCClient) Post(method string,arg interface{}) error {

	req := &RPCRequest{} 
	req.Method = method
	req.Seq = atomic.AddUint64(&this.sequence,1) 
	req.Arg = arg
	req.NeedResp = false

	request,err := this.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("encode error:%s\n",err.Error())
	} 

	err = this.channel.SendRequest(request)
	if nil != err {
		return err
	}
	return nil
}


func AsynHandler(cb RPCResponseHandler) RPCResponseHandler {
	if nil != cb {
		return func (r interface{},e error) {
			go cb(r,e)
		}
	} else {
		return nil
	}
}

/*
*  异步调用
*  cb将在一个单独的go程中执行,如需在cb中调用阻塞函数请使用AsynHandler封装cb
*/
func (this *RPCClient) AsynCall(method string,arg interface{},timeout uint32,cb RPCResponseHandler) error {

	if cb == nil {
		return fmt.Errorf("cb == nil")
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

	err = this.channel.SendRequest(request)
	if nil != err {
		return err
	}
	r := &reqContext{seq:req.Seq,onResponse:cb}
	if timeout > 0 {
		r.deadline = time.Now().Add(time.Duration(timeout) * time.Millisecond)
	}
	reqMgr.pushRequest(this.channel,r)
	return nil
}

//同步调用
func (this *RPCClient) SyncCall(method string,arg interface{},timeout uint32) (interface{},error) {
	type resp struct {
		ret interface{}
		err error
	}

	respChan := make(chan *resp)

	f := func (ret interface{},err error) {
		respChan <- &resp{ret:ret,err:err}
	}
	
	if err := this.AsynCall(method,arg,timeout,f); err != nil {
		return nil,err
	}

	result := <- respChan
	return result.ret,result.err
}

func NewClient(channel RPCChannel,decoder RPCMessageDecoder,encoder RPCMessageEncoder) (*RPCClient,error) {
	if nil == decoder {
		return nil,fmt.Errorf("decoder == nil")
	}

	if nil == encoder {
		return nil,fmt.Errorf("encoder == nil")
	}

	if nil == channel {
		return nil,fmt.Errorf("channel == nil")
	}

	return &RPCClient{encoder:encoder,decoder:decoder,channel:channel},nil
}

func init() {

	reqMgr = &reqContextMgr{queue:util.NewBlockQueue(),channels:map[RPCChannel]*channelContext{}}

	//启动一个go程，每10毫秒向queue投递一个定时器消息
	go func() {
		for {
			time.Sleep(time.Duration(10)*time.Millisecond)
			reqMgr.pushTimer()
		}
	}()

	go func() {
		reqMgr.loop()
	}()

}
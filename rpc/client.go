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


type RPCClient struct {
	encoder   		  	RPCMessageEncoder
	decoder   		  	RPCMessageDecoder
	sequence            uint64
	mutex             	sync.Mutex
	channel             RPCChannel
	pendingCalls        map[uint64]*reqContext        //等待回复的请求
	minheap            *util.MinHeap 
	timeoutRoutine      bool              
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
				this.startTimeoutRoutine()
			}
		}
	}
	this.mutex.Unlock()
	return nil
}

//同步调用
func (this *RPCClient) SyncCall(method string,arg interface{},timeout uint32) (interface{},error) {
	return nil,nil
}


var ErrCallTimeout error = fmt.Errorf("rpc call timeout")

//启动一个goroutine检测call超时
func (this *RPCClient) startTimeoutRoutine() {
	if this.timeoutRoutine {
		return
	}
	this.timeoutRoutine = true

	go func() {
		defer func(){
			this.mutex.Unlock()
		}()
		this.mutex.Lock()
		for {
			now := time.Now()
			sleepTime := int64(0)
			r := this.minheap.Min()
			if nil != r {
				sleepTime = r.(*reqContext).deadline.UnixNano() - now.UnixNano()
			}

			if sleepTime > 0 {
				this.mutex.Unlock()
				time.Sleep(time.Duration(sleepTime))
				this.mutex.Lock()
				now = time.Now()				
			}

			for {
				r = this.minheap.Min()
				if nil == r || !now.After(r.(*reqContext).deadline) {
					break
				}
				this.minheap.PopMin()
				go func() {
					r.(*reqContext).onResponse(nil,ErrCallTimeout)
				}()
 			}
 			if this.channel == nil {
 				return
 			}
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
	c.minheap = util.NewMinHeap(64)
	c.channel = channel
	return c,nil
}
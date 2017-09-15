package rpc

import (
	rpc_channel "github.com/sniperHW/kendynet/rpc/channel"
	"sync"
	"fmt"
	"sync/atomic"
	"runtime"
)

const (
	RPC_REQUEST = 1
	RPC_RESPONSE = 2
)

var sequence uint64 = 0

type RPCMessage interface {
	Type() byte
}

type RPCRequest struct {
	seq    uint64
	method string
	arg    interface{}
}

type RPCResponse struct {
	seq    uint64
	err    error
	ret    interface{}
}

func (this *RPCRequest) Type() byte {
	return RPC_REQUEST
}

func (this *RPCResponse) Type() byte {
	return RPC_RESPONSE
}

type RPCMessageEncoder interface {
	Encode(RPCMessage) (interface{},error)
}

type RPCMessageDecoder interface {
	Decode(interface{}) (RPCMessage,error)	
}

//调用上下文
type channelContext struct {
	seq uint64
	request interface{}   //保存解包后的请求用于将来重传
	onResponse func(interface{},error)
}

func (this *channelContext) sendRequest(channel rpc_channel.RPCChannel) error {
	return channel.SendRPCMessage(this.request)
}

type channelContextMgr struct {
	channel  rpc_channel.RPCChannel
	contexts map[uint64]*channelContext
}

func (this *channelContextMgr) getAndRemoveContext(seq uint64) *channelContext {
	context,ok := this.contexts[seq]
	if !ok {
		return nil
	} else {
		delete(this.contexts,seq)
		return context
	}
}

func (this *channelContextMgr) onChannelDisconnected(err error) {
	for _,context := range this.contexts {
		context.onResponse(nil,err)
	}
}

type RPCManager struct {
	encoder   		 RPCMessageEncoder
	decoder   		 RPCMessageDecoder
	methods   		 map[string]func (interface{})(interface{},error)
	mutexMethods     sync.Mutex
	contexts         map[string]*channelContextMgr
	mutexContexts    sync.Mutex
}

func NewRPCManager(decoder  RPCMessageDecoder,encoder RPCMessageEncoder) (*RPCManager,error) {
	if nil == decoder {
		return nil,fmt.Errorf("decoder == nil")
	}

	if nil == encoder {
		return nil,fmt.Errorf("encoder == nil")
	}

	return &RPCManager{decoder:decoder,encoder:encoder},nil
}

func (this *RPCManager) RegisterService(name string,service func (interface{})(interface{},error)) error {
	if name == "" {
		return fmt.Errorf("name == ''")
	}

	if nil == service {
		return fmt.Errorf("service == nil")		
	}

	defer func(){
		this.mutexMethods.Unlock()
	}()
	this.mutexMethods.Lock()

	_,ok := this.methods[name]
	if ok {
		return fmt.Errorf("duplicate method:%s",name)
	} 
	this.methods[name] = service
	return nil
}

func (this *RPCManager) UnRegisterService(name string) {
	defer func(){
		this.mutexMethods.Unlock()
	}()
	this.mutexMethods.Lock()
	delete(this.methods,name)	
}

func (this *RPCManager) sendMessage(to rpc_channel.RPCChannel,message RPCMessage) {
	msg,err := this.encoder.Encode(message)
	if nil != err {
		fmt.Printf("Encode rpc message error:%s\n",err.Error())
		return
	}
	err = to.SendRPCMessage(msg)
	if nil != err {
		fmt.Printf("send rpc message to (%s) error:%s\n",to.Name() , err.Error())		
	}
}

func (this *RPCManager) callService(method func(interface{})(interface{},error),arg interface{}) (ret interface{},err error) {
	defer func(){
		if r := recover(); r != nil {
			buf := make([]byte, 65535)
			l := runtime.Stack(buf, false)
			ret = nil
			err = fmt.Errorf("%v: %s", r, buf[:l])
		}			
	}()
	ret,err = method(arg)
	return
}

func (this *RPCManager) onRPCRequest(from rpc_channel.RPCChannel, req *RPCRequest) {
	this.mutexMethods.Lock()
	method,ok := this.methods[req.method]
	this.mutexMethods.Unlock()
	if !ok {
		err := fmt.Errorf("invaild method:%s",req.method)
		response := &RPCResponse{seq:req.seq,err:err}
		this.sendMessage(from,response)
		fmt.Printf("rpc request from(%s) invaild method %s\n",from.Name(),req.method)
		return		
	}

	ret,err := this.callService(method,req.arg)
	response := &RPCResponse{seq:req.seq,err:err,ret:ret}
	this.sendMessage(from,response)
}

func (this *RPCManager) OnChannelDisconnected(channel rpc_channel.RPCChannel,reason string) {
	this.mutexContexts.Lock()
	contextMgr,ok := this.contexts[channel.Name()] 
	if !ok {
		this.mutexContexts.Unlock()
		return
	}
	delete(this.contexts,channel.Name())
	this.mutexContexts.Unlock()
	contextMgr.onChannelDisconnected(fmt.Errorf(reason))
}

func (this *RPCManager) onRPCResponse(from rpc_channel.RPCChannel, r *RPCResponse) {
	this.mutexContexts.Lock()
	contextMgr,ok := this.contexts[from.Name()] 
	if !ok {
		//记录日志
		this.mutexContexts.Unlock()
		return
	}
	context := contextMgr.getAndRemoveContext(r.seq)
	this.mutexContexts.Unlock()	

	if nil == context {
		//记录日志
	} else {
		context.onResponse(r.ret,r.err)
	}
}

func (this *RPCManager) OnRPCMessage(channel rpc_channel.RPCChannel,message interface{}) {
	msg,err := this.decoder.Decode(message)
	if nil != err {
		fmt.Printf("rpc message from(%s) decode err:%s\n",channel.Name,err.Error())
		return
	}
	if msg.Type() == RPC_REQUEST {
		this.onRPCRequest(channel,msg.(*RPCRequest))
	} else if msg.Type() == RPC_RESPONSE {
		this.onRPCResponse(channel,msg.(*RPCResponse))
	} else {
		fmt.Printf("rpc message from(%s) invaild type:%s\n",channel.Name,msg.Type())
	}
}

/*
*   原始网络模型是在接收线程中执行消息回调，如果使用同步调用会导致接收线程阻塞(导致无法接收到响应)
*   所以只提供异步调用接口，如果使用者将消息处理提到单独的线程，使用者可按需封装同步调用接口
*/

func (this *RPCManager) Call(channel rpc_channel.RPCChannel,service string,arg interface{},cb func(interface{},error)) error {

	if channel == nil {
		return fmt.Errorf("channel == nil")
	}

	if cb == nil {
		return fmt.Errorf("cb == nil")
	}
	
	req := &RPCRequest{} 
	req.method = service
	req.seq = atomic.AddUint64(&sequence,1) 
	context := &channelContext{}
	var err error
	context.request,err = this.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("arg encode error:%s\n",err.Error())
	}
	context.seq = req.seq 
	context.onResponse = cb

	this.mutexContexts.Lock()
	defer func(){
		this.mutexContexts.Unlock()	
	}()

	/* 必须在加锁的环境下发送,因为环境是多线程的，存在发送完成后将context加入contextMgr之前收到response
	*  如果没有锁的保护，response将找不到对应的context
	*/
	err = context.sendRequest(channel)
	if nil != err {
		this.mutexContexts.Unlock()
		return err
	}
	contextMgr,ok := this.contexts[channel.Name()] 
	if !ok {
		contextMgr = &channelContextMgr{}
		contextMgr.channel = channel
		this.contexts[channel.Name()] = contextMgr
	}
	return nil
}
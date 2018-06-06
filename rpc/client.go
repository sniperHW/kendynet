package rpc

import (
	"sync"
	"fmt"
	"sync/atomic"
	"time"
	"github.com/sniperHW/kendynet/util"
)

var sequence uint64 = 0
var ErrCallTimeout error = fmt.Errorf("rpc call timeout")
var ErrNoService   error = fmt.Errorf("no available service")

type Option struct {
	Timeout 		 time.Duration                //调用超时的时间,毫秒 0 <= 为不超时
}

/*
*     channel选择策略接口
*/
type ChannelSelector interface {
	Select([]RPCChannel) RPCChannel
}

type RPCResponseHandler func(interface{},error)

type reqContext struct {
	chanContext *channelContext
	next        *reqContext
	pre         *reqContext
	seq         uint64
	onResponse  RPCResponseHandler
	deadline    time.Time
}

type channelContext struct {
	channel     RPCChannel
	methods     map[string]bool               //channel上提供的远程方法
	pendingReqs map[uint64]*reqContext        //等待回复的请求
}

type reqQueue struct {
	head  reqContext
	tail  reqContext
	count uint64
}

func (this *reqQueue) init() {
	this.head.next = &this.tail
	this.tail.pre  = &this.head 
}

func (this *reqQueue) front() *reqContext {
	if this.count == 0 {
		return nil
	} else {
		return this.head.next 
	}
}

func (this *reqQueue) popFront() *reqContext {
	if this.count == 0 {
		return nil
	} else {
		f := this.head.next
		this.head.next = f.next
		f.next.pre = &this.head
		f.next = nil
		f.pre  = nil
		this.count--
		return f 
	}	
}

func (this *reqQueue) remove(p *reqContext) {
	pre  := p.pre
	next := p.next
	pre.next = next
	next.pre = pre
	this.count--
	p.pre = nil
	p.next = nil
}

func (this *reqQueue) push(p *reqContext) {
	if p.next == nil && p.pre == nil {
		pre := this.tail.pre
		pre.next = p
		p.pre = pre
		p.next = &this.tail
		this.tail.pre = p
		this.count++
	}
}

//提供method的所有channel
type methodChannels struct {
	channels    []RPCChannel
}

func (this *methodChannels) addChannel(channel RPCChannel) {
	if this.channels == nil {
		this.channels = make([]RPCChannel,0,10)
	}
	this.channels = append(this.channels,channel)
}

func (this *methodChannels) removeChannel(channel RPCChannel) {
	if this.channels == nil {
		return
	}

	for i := 0; i < len(this.channels); i++ {
		if this.channels[i] == channel {
			//将后面的元素往前移动
			for j := i; j + 1 < len(this.channels); j++ {
				this.channels[j] = this.channels[j+1]
			}
			this.channels = this.channels[:len(this.channels)-1]
			return 
		}
	}
}

type RPCClient struct {
	encoder   		  	RPCMessageEncoder
	decoder   		  	RPCMessageDecoder
	option            	Option
	mutex             	sync.Mutex
	pendingCalls      	reqQueue    				//按deadlie从小到大排列的未收到应答的call请求
	checkTimeoutRoutine bool
	mehtodChannelsMap   map[string]*methodChannels   
	channelNameMap      map[string]RPCChannel
	channels            map[RPCChannel]*channelContext
}

//添加一个远程方法的通道
func (this *RPCClient) addMethodChannel(method string,channel RPCChannel) {
	m,ok := this.mehtodChannelsMap[method]
	if !ok {
		m = &methodChannels{}
		this.mehtodChannelsMap[method] = m
	}
	m.addChannel(channel)
}

//移除一个远程方法的通道
func (this *RPCClient) removeMethodChannel(method string,channel RPCChannel) {
	m,ok := this.mehtodChannelsMap[method]
	if !ok {
		return
	}
	m.removeChannel(channel)	
}

//在channel上添加method
func (this *RPCClient) AddMethod(channel RPCChannel,method string) {
	if nil == channel {
		return
	}

	defer func(){
		this.mutex.Unlock()
	}()
	this.mutex.Lock()

	_,ok := this.channelNameMap[channel.Name()]
	if !ok {
		this.channelNameMap[channel.Name()] = channel
		context := &channelContext{}
		context.methods = make(map[string]bool)
		context.pendingReqs = make(map[uint64]*reqContext)
		context.methods[method] = true
		context.channel = channel
		this.channels[channel] = context
		this.addMethodChannel(method,channel)
	} else {
		context := this.channels[channel]
		_,ok = context.methods[method]
		if ok {
			return
		}
		context.methods[method] = true
		this.addMethodChannel(method,channel)
	}
}

//在channel上移除method
func (this *RPCClient) RemoveMethod(channel RPCChannel,method string) {
	if nil == channel {
		return
	}

	defer func(){
		this.mutex.Unlock()
	}()
	this.mutex.Lock()

	_,ok := this.channelNameMap[channel.Name()]
	if !ok {
		return
	}
	context := this.channels[channel]
	delete(context.methods,method)
	this.removeMethodChannel(method,channel)
}

//启动一个goroutine检测call超时
func (this *RPCClient) startReqTimeoutCheckRoutine() {
	if this.checkTimeoutRoutine {
		return
	}
	this.checkTimeoutRoutine = true
	go func(){
		defer func(){
			this.mutex.Unlock()
		}()
		this.mutex.Lock()
		timeoutList := reqQueue{}
		timeoutList.init()
		for {
			now := time.Now()
			sleepTime := int64(0)
			if this.option.Timeout > 0 {
				p := this.pendingCalls.front()
				if p != nil {
					sleepTime = p.deadline.UnixNano() - now.UnixNano()
				} else {
					sleepTime = int64(this.option.Timeout)
				}
			}

			if sleepTime > 0 {
				this.mutex.Unlock()
				time.Sleep(time.Duration(sleepTime))
				this.mutex.Lock()
				now = time.Now()				
			}

			for this.pendingCalls.count > 0 {
				p := this.pendingCalls.front()
				if now.After(p.deadline) {
					this.pendingCalls.popFront()
					delete(p.chanContext.pendingReqs,p.seq)
					timeoutList.push(p)
				} else {
					break
				}
			}

			if timeoutList.count > 0 {
				this.mutex.Unlock()
				for timeoutList.count > 0 {
					p := timeoutList.popFront()
					p.onResponse(nil,ErrCallTimeout)
				}
				this.mutex.Lock()
			}

			if len(this.channels) == 0 {
				this.checkTimeoutRoutine = false
				return
			}
		}
	}()
}


func (this *RPCClient) OnChannelClose(channel RPCChannel,err error) {
	if nil == channel {
		return
	}

	respList := reqQueue{}
	respList.init()

	defer func(){
		this.mutex.Unlock()
		for respList.count > 0 {
			p := respList.popFront()
			p.onResponse(nil,err)
		}
	}()
	this.mutex.Lock()

	_,ok := this.channelNameMap[channel.Name()]
	if !ok {
		return
	}
	delete(this.channelNameMap,channel.Name())
	chanContext,ok := this.channels[channel]
	if !ok {
		return
	}

	for _,r := range chanContext.pendingReqs {
		respList.push(r)
		this.pendingCalls.remove(r)
	}

	for m,_ := range chanContext.methods {
		this.removeMethodChannel(m,channel)
	}

	delete(this.channels,channel)

}

func (this *RPCClient) OnRPCMessage(channel RPCChannel,message interface{}) {
	msg,err := this.decoder.Decode(message)
	if nil != err {
		Errorf(util.FormatFileLine("RPCClient rpc message from(%s) decode err:%s\n",channel.Name,err.Error()))
		return
	}

	this.mutex.Lock()
	chanContext,ok := this.channels[channel]
	if !ok {
		this.mutex.Unlock()
		return
	}
	pendingReq,ok := chanContext.pendingReqs[msg.GetSeq()] 
	if !ok {
		this.mutex.Unlock()
		return
	}
	delete(chanContext.pendingReqs,msg.GetSeq())
	this.pendingCalls.remove(pendingReq)
	
	this.mutex.Unlock()

	resp := msg.(*RPCResponse)
	pendingReq.onResponse(resp.Ret,resp.Err)
	
}


func (this *RPCClient) Call(selector ChannelSelector,method string,arg interface{},cb RPCResponseHandler) error {
	if cb == nil {
		return fmt.Errorf("cb == nil")
	}
	
	req := &RPCRequest{} 
	req.Method = method
	req.Seq = atomic.AddUint64(&sequence,1) 
	req.Arg = arg
	req.NeedResp = true

	request,err := this.encoder.Encode(req)
	if err != nil {
		return fmt.Errorf("encode error:%s\n",err.Error())
	} 

	this.mutex.Lock()
	defer func(){
		this.mutex.Unlock()	
	}()

	channels,ok := this.mehtodChannelsMap[method]
	if !ok {
		return ErrNoService
	} 

	channel := selector.Select(channels.channels)

	if nil == channel {
		return ErrNoService
	}

	err = channel.SendRPCRequest(request)
	if nil != err {
		return err
	}

	chanContext := this.channels[channel]
	r := &reqContext{seq:req.Seq,chanContext:chanContext,onResponse:cb}
	chanContext.pendingReqs[req.Seq] = r
	if this.option.Timeout > 0 {
		r.deadline = time.Now().Add(this.option.Timeout)
		this.pendingCalls.push(r)
		if !this.checkTimeoutRoutine {
			this.startReqTimeoutCheckRoutine()
		}
	}

	return nil	
}

func NewRPCClient(decoder RPCMessageDecoder,encoder RPCMessageEncoder,option *Option) (*RPCClient,error) {
	if nil == decoder {
		return nil,fmt.Errorf("decoder == nil")
	}

	if nil == encoder {
		return nil,fmt.Errorf("encoder == nil")
	}

	if nil == option {
		option = &Option{}
	}

	c := &RPCClient{}
	c.encoder = encoder
	c.decoder = decoder
	c.option  = *option
	c.pendingCalls.init()
	c.mehtodChannelsMap = make(map[string]*methodChannels)
	c.channelNameMap = make(map[string]RPCChannel)
	c.channels = make(map[RPCChannel]*channelContext)
	return c,nil
}


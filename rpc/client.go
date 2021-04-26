package rpc

import (
	"errors"
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/util"
	"sync"
	"sync/atomic"
	"time"
)

var ErrCallTimeout error = fmt.Errorf("rpc call timeout")
var ErrChannelDisconnected error = fmt.Errorf("channel disconnected")

var sequence uint64

type RPCResponseHandler func(interface{}, error)

type callContext struct {
	seq           uint64
	channelID     uint64
	onResponse    RPCResponseHandler
	deadlineTimer *time.Timer
	rpcCli        *RPCClient
}

var callContextPool = sync.Pool{
	New: func() interface{} {
		return &callContext{}
	},
}

func getCallContext() *callContext {
	return callContextPool.Get().(*callContext)
}

func releaseCallContext(c *callContext) {
	callContextPool.Put(c)
}

type channelCalls struct {
	calls map[uint64]*callContext
}

type RPCClient struct {
	encoder RPCMessageEncoder
	decoder RPCMessageDecoder

	mu           sync.Mutex
	callContexts map[uint64]*callContext
	channels     map[uint64]*channelCalls
}

func (this *callContext) onTimeout() {
	if nil != this.rpcCli.removeCallBySeqno(this.seq) {
		this.onResponse(nil, ErrCallTimeout)
		releaseCallContext(this)
	}
}

func (this *RPCClient) addCall(call *callContext, timeout time.Duration) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.callContexts[call.seq] = call
	cc, ok := this.channels[call.channelID]
	if !ok {
		cc = &channelCalls{
			calls: map[uint64]*callContext{},
		}
		this.channels[call.channelID] = cc
	}
	cc.calls[call.seq] = call
	call.deadlineTimer = time.AfterFunc(timeout, call.onTimeout)
}

func (this *RPCClient) removeCallBySeqno(seq uint64) *callContext {
	this.mu.Lock()
	defer this.mu.Unlock()
	if call, ok := this.callContexts[seq]; ok {
		cc := this.channels[call.channelID]
		delete(cc.calls, call.channelID)
		if len(cc.calls) == 0 {
			delete(this.channels, call.channelID)
		}
		call.deadlineTimer.Stop()
		return call
	}
	return nil
}

func (this *RPCClient) OnRPCMessage(message interface{}) {
	if msg, err := this.decoder.Decode(message); nil != err {
		kendynet.GetLogger().Errorf(util.FormatFileLine("RPCClient rpc message decode err:%s\n", err.Error()))
	} else {
		if resp, ok := msg.(*RPCResponse); ok {
			if call := this.removeCallBySeqno(resp.GetSeq()); nil != call {
				call.onResponse(resp.Ret, resp.Err)
				releaseCallContext(call)
			} else {
				kendynet.GetLogger().Info("onResponse with no reqContext", resp.GetSeq())
			}
		}
	}
}

func (this *RPCClient) OnChannelDisconnect(channel RPCChannel) {
	this.mu.Lock()
	cc, ok := this.channels[channel.UID()]
	if ok {
		delete(this.channels, channel.UID())
	}
	this.mu.Unlock()

	for k, v := range cc.calls {
		this.mu.Lock()
		if _, ok = this.callContexts[k]; ok {
			delete(this.callContexts, k)
		}
		this.mu.Unlock()
		if ok {
			v.deadlineTimer.Stop()
			v.onResponse(nil, ErrChannelDisconnected)
			releaseCallContext(v)
		}
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

	if request, err := this.encoder.Encode(req); nil != err {
		return fmt.Errorf("encode error:%s\n", err.Error())
	} else {
		if err = channel.SendRequest(request); nil != err {
			return err
		} else {
			return nil
		}
	}
}

func (this *RPCClient) AsynCall(channel RPCChannel, method string, arg interface{}, timeout time.Duration, cb RPCResponseHandler) error {

	if cb == nil {
		return errors.New("cb == nil")
	}

	req := &RPCRequest{
		Method:   method,
		Seq:      atomic.AddUint64(&sequence, 1),
		Arg:      arg,
		NeedResp: true,
	}

	if request, err := this.encoder.Encode(req); err != nil {
		return err
	} else {
		context := getCallContext()
		context.onResponse = cb
		context.seq = req.Seq
		context.rpcCli = this
		context.channelID = channel.UID()
		this.addCall(context, timeout)
		if err = channel.SendRequest(request); err == nil {
			return nil
		} else {
			if nil != this.removeCallBySeqno(context.seq) {
				releaseCallContext(context)
			}
			return err
		}
	}
}

//同步调用
func (this *RPCClient) Call(channel RPCChannel, method string, arg interface{}, timeout time.Duration) (ret interface{}, err error) {
	waitC := make(chan struct{})
	f := func(ret_ interface{}, err_ error) {
		ret = ret_
		err = err_
		close(waitC)
	}

	if err = this.AsynCall(channel, method, arg, timeout, f); nil == err {
		<-waitC
	}

	return
}

func NewClient(decoder RPCMessageDecoder, encoder RPCMessageEncoder) *RPCClient {
	if nil == decoder || nil == encoder {
		return nil
	} else {

		c := &RPCClient{
			encoder:      encoder,
			decoder:      decoder,
			callContexts: map[uint64]*callContext{},
			channels:     map[uint64]*channelCalls{},
		}
		return c
	}
}

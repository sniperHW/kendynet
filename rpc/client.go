package rpc

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/timer"
	"github.com/sniperHW/kendynet/util"
	//"sync"
	"sync/atomic"
	"time"
)

var ErrCallTimeout error = fmt.Errorf("rpc call timeout")

var sequence uint64

type RPCResponseHandler func(interface{}, error)

type contextGroup struct {
	timerMgr *timer.TimerMgr
}

func (this *contextGroup) onResponse(resp *RPCResponse) {
	var ok bool
	var ctx interface{}
	if ok, ctx = this.timerMgr.CancelByIndex(resp.GetSeq()); ok {
		ctx.(*reqContext).callResponseCB(resp.Ret, resp.Err)
	} else if nil == ctx {
		kendynet.GetLogger().Infoln("onResponse with no reqContext", resp.GetSeq())
	}
}

type reqContext struct {
	seq          uint64
	onResponse   RPCResponseHandler
	cbEventQueue *event.EventQueue
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

func (this *reqContext) onTimeout(_ *timer.Timer, _ interface{}) {
	kendynet.GetLogger().Infoln("req timeout", this.seq)
	this.callResponseCB(nil, ErrCallTimeout)
}

type RPCClient struct {
	encoder       RPCMessageEncoder
	decoder       RPCMessageDecoder
	cbEventQueue  *event.EventQueue
	contextGroups []*contextGroup
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	if msg, err := this.decoder.Decode(message); nil != err {
		kendynet.GetLogger().Errorf(util.FormatFileLine("RPCClient rpc message decode err:%s\n", err.Error()))
	} else {
		if resp, ok := msg.(*RPCResponse); ok {
			this.contextGroups[msg.GetSeq()%uint64(len(this.contextGroups))].onResponse(resp)
		} else {
			panic("RPCClient.OnRPCMessage() invaild msg type")
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

	if request, err := this.encoder.Encode(req); err != nil {
		return err
	} else {
		group := this.contextGroups[req.Seq%uint64(len(this.contextGroups))]
		group.timerMgr.OnceWithIndex(timeout, nil, context.onTimeout, context, context.seq)
		if err = channel.SendRequest(request); err == nil {
			return nil
		} else {
			group.timerMgr.CancelByIndex(context.seq)
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

	if err = this.AsynCall(channel, method, arg, timeout, f); nil == err {
		_ = <-respChan
	}

	return
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

	c := &RPCClient{
		encoder:       encoder,
		decoder:       decoder,
		cbEventQueue:  q,
		contextGroups: make([]*contextGroup, 61),
	}

	for i, _ := range c.contextGroups {
		c.contextGroups[i] = &contextGroup{
			timerMgr: timer.NewTimerMgr(1),
		}
	}

	return c
}

/*

package rpc

import (
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	"github.com/sniperHW/kendynet/timer"
	"github.com/sniperHW/kendynet/util"
	"sync"
	"sync/atomic"
	"time"
)

var ErrCallTimeout error = fmt.Errorf("rpc call timeout")
var sequence uint64

type RPCResponseHandler func(interface{}, error)

type reqContext struct {
	seq          uint64
	onResponse   RPCResponseHandler
	cbEventQueue *event.EventQueue
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

func (this *reqContext) onTimeout(_ *timer.Timer, _ interface{}) {
	kendynet.GetLogger().Infoln("req timeout", this.seq)
	this.callResponseCB(nil, ErrCallTimeout)
}

type RPCClient struct {
	encoder      RPCMessageEncoder
	decoder      RPCMessageDecoder
	cbEventQueue *event.EventQueue
	timerMgrs    []*timer.TimerMgr
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	if msg, err := this.decoder.Decode(message); nil != err {
		kendynet.GetLogger().Errorf(util.FormatFileLine("RPCClient rpc message decode err:%s\n", err.Error()))
	} else {
		if resp, ok := msg.(*RPCResponse); ok {
			mgr := this.timerMgrs[msg.GetSeq()%uint64(len(this.timerMgrs))]
			if ok, ctx := mgr.CancelByIndex(resp.GetSeq()); ok {
				ctx.(*reqContext).callResponseCB(resp.Ret, resp.Err)
			} else if nil == ctx {
				kendynet.GetLogger().Infoln("onResponse with no reqContext", resp.GetSeq())
			}
		} else {
			panic("RPCClient.OnRPCMessage() invaild msg type")
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

	if request, err := this.encoder.Encode(req); err != nil {
		return err
	} else {
		mgr := this.timerMgrs[req.Seq%uint64(len(this.timerMgrs))]
		mgr.OnceWithIndex(timeout, nil, context.onTimeout, context, context.seq)
		if err = channel.SendRequest(request); err == nil {
			return nil
		} else {
			mgr.CancelByIndex(context.seq)
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

	if err = this.AsynCall(channel, method, arg, timeout, f); nil == err {
		_ = <-respChan
	}

	return
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

	c := &RPCClient{
		encoder:      encoder,
		decoder:      decoder,
		cbEventQueue: q,
		timerMgrs:    make([]*timer.TimerMgr, 61),
	}
	for i, _ := range c.timerMgrs {
		c.timerMgrs[i] = timer.NewTimerMgr(1)
	}

	return c
}
*/

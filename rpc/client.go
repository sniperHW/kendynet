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

var client_once sync.Once
var sequence uint64
var groupCount uint64 = 61
var contextGroups []*contextGroup = make([]*contextGroup, groupCount)

type RPCResponseHandler func(interface{}, error)

type contextGroup struct {
	mtx      sync.Mutex
	waitResp map[uint64]*reqContext
	timerMgr *timer.TimerMgr
}

func (this *contextGroup) onResponse(resp *RPCResponse) {
	var (
		ctx *reqContext
		ok  bool
	)

	seq := resp.GetSeq()

	this.mtx.Lock()
	if ctx, ok = this.waitResp[seq]; ok {
		if ctx.timer.Cancel() {
			delete(this.waitResp, seq)
		} else {
			this.mtx.Unlock()
			return
		}
	}
	this.mtx.Unlock()

	if nil != ctx {
		ctx.callResponseCB(resp.Ret, resp.Err)
	} else {
		kendynet.Debugf("on response,but missing reqContext:%d\n", seq)
	}
}

type reqContext struct {
	seq          uint64
	onResponse   RPCResponseHandler
	cbEventQueue *event.EventQueue
	timer        *timer.Timer
	group        *contextGroup
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
	ok := false
	this.group.mtx.Lock()
	if _, ok := this.group.waitResp[this.seq]; !ok {
		kendynet.Infof("timeout context:%d not found\n", this.seq)
	} else {
		ok = true
		delete(this.group.waitResp, this.seq)
		kendynet.Infof("timeout context:%d\n", this.seq)
	}
	this.group.mtx.Unlock()
	if ok {
		this.callResponseCB(nil, ErrCallTimeout)
	}
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
	contextGroups[msg.GetSeq()%uint64(len(contextGroups))].onResponse(msg.(*RPCResponse))
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
		group := contextGroups[req.Seq%uint64(len(contextGroups))]
		group.mtx.Lock()
		defer group.mtx.Unlock()
		err := channel.SendRequest(request)
		if err == nil {
			group.waitResp[context.seq] = context
			context.timer = group.timerMgr.Once(timeout, nil, context.onTimeout, nil)
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
		for i := uint64(0); i < groupCount; i++ {
			contextGroups[i] = &contextGroup{
				timerMgr: timer.NewTimerMgr(),
				waitResp: map[uint64]*reqContext{},
			}
		}
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

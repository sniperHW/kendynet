package rpc

import (
	"container/list"
	"errors"
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/timer"
	"github.com/sniperHW/kendynet/util"
	"sync"
	"sync/atomic"
	"time"
)

var ErrCallTimeout error = fmt.Errorf("rpc call timeout")
var ErrChannelDisconnected error = fmt.Errorf("channel disconnected")

type RPCResponseHandler func(interface{}, error)

type reqContext struct {
	seq        uint64
	onResponse RPCResponseHandler
	listEle    *list.Element
	channelUID uint64
	c          *RPCClient
}

var reqContextPool = sync.Pool{
	New: func() interface{} {
		return &reqContext{}
	},
}

type channelReqContexts struct {
	reqs *list.List
}

func (this *channelReqContexts) add(req *reqContext) {
	req.listEle = this.reqs.PushBack(req)
}

func (this *channelReqContexts) remove(req *reqContext) bool {
	if nil != req.listEle {
		this.reqs.Remove(req.listEle)
		req.listEle = nil
		reqContextPool.Put(req)
		return true
	} else {
		return false
	}
}

func onReqTimeout(_ *timer.Timer, v interface{}) {
	c := v.(*reqContext)
	kendynet.GetLogger().Info("req timeout", c.seq, time.Now())
	onResponse := c.onResponse
	cli := c.c
	if cli.removeChannelReq(c) {
		onResponse(nil, ErrCallTimeout)
	}
}

type channelReqMap struct {
	sync.Mutex
	m map[uint64]*channelReqContexts
}

type RPCClient struct {
	encoder        RPCMessageEncoder
	decoder        RPCMessageDecoder
	channelReqMaps []channelReqMap
	sequence       uint64
	indexMgr       timer.IndexMgr
}

func (this *RPCClient) addChannelReq(channel RPCChannel, req *reqContext) {
	uid := channel.UID()
	m := this.channelReqMaps[int(uid)%len(this.channelReqMaps)]

	m.Lock()
	defer m.Unlock()
	c, ok := m.m[uid]
	if !ok {
		c = &channelReqContexts{
			reqs: list.New(),
		}
		m.m[uid] = c
	}
	c.add(req)
}

func (this *RPCClient) removeChannelReq(req *reqContext) bool {
	uid := req.channelUID
	m := this.channelReqMaps[int(uid)%len(this.channelReqMaps)]

	m.Lock()
	defer m.Unlock()

	c, ok := m.m[uid]
	if ok {
		ret := c.remove(req)
		if c.reqs.Len() == 0 {
			delete(m.m, uid)
		}
		return ret
	} else {
		return false
	}
}

func (this *RPCClient) OnChannelDisconnect(channel RPCChannel) {
	uid := channel.UID()
	m := this.channelReqMaps[int(uid)%len(this.channelReqMaps)]

	m.Lock()
	channelReqContexts, ok := m.m[uid]
	if ok {
		delete(m.m, uid)
	}
	m.Unlock()

	if ok {
		for {
			if v := channelReqContexts.reqs.Front(); nil != v {
				c := v.Value.(*reqContext)

				/*
				 * 不管CancelByIndex是否返回true都应该调用onResponse
				 * 考虑如下onResponse调用丢失的场景
				 * A线程执行OnChannelDisconnect,走到m.Unlock()之后的代码
				 * B线程执行onTimeout的removeChannelReq,此时removeChannelReq必然返回false,因此onResponse不会执行。
				 * A线程继续执行,如果像OnRPCMessage一样判断CancelByIndex为true才执行onResponse,那么对于这个请求的onResponse将丢失
				 * 因为这个req的onTimeout已经被执行，CancelByIndex必定返回false
				 */
				this.indexMgr.CancelByIndex(c.seq)
				c.onResponse(nil, ErrChannelDisconnected)
				c.listEle = nil

				channelReqContexts.reqs.Remove(v)
				reqContextPool.Put(c)
			} else {
				break
			}
		}
	}
}

//收到RPC消息后调用
func (this *RPCClient) OnRPCMessage(message interface{}) {
	if msg, err := this.decoder.Decode(message); nil != err {
		kendynet.GetLogger().Errorf(util.FormatFileLine("RPCClient rpc message decode err:%s\n", err.Error()))
	} else {
		if resp, ok := msg.(*RPCResponse); ok {
			if ok, ctx := this.indexMgr.CancelByIndex(resp.GetSeq()); ok {
				if this.removeChannelReq(ctx.(*reqContext)) {
					ctx.(*reqContext).onResponse(resp.Ret, resp.Err)
				}
			} else if nil == ctx {
				kendynet.GetLogger().Info("onResponse with no reqContext", resp.GetSeq())
			}
		}
	}
}

//投递，不关心响应和是否失败
func (this *RPCClient) Post(channel RPCChannel, method string, arg interface{}) error {

	req := &RPCRequest{
		Method:   method,
		Seq:      atomic.AddUint64(&this.sequence, 1),
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
		Seq:      atomic.AddUint64(&this.sequence, 1),
		Arg:      arg,
		NeedResp: true,
	}

	if request, err := this.encoder.Encode(req); err != nil {
		return err
	} else {
		context := reqContextPool.Get().(*reqContext)
		context.onResponse = cb
		context.seq = req.Seq
		context.c = this
		context.channelUID = channel.UID()

		this.addChannelReq(channel, context)
		this.indexMgr.OnceWithIndex(timeout, onReqTimeout, context, context.seq)
		if err = channel.SendRequest(request); err == nil {
			return nil
		} else {
			this.removeChannelReq(context)
			this.indexMgr.CancelByIndex(req.Seq)
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
			encoder: encoder,
			decoder: decoder,
		}

		for i := 0; i < 127; i++ {
			c.channelReqMaps = append(c.channelReqMaps, channelReqMap{
				m: map[uint64]*channelReqContexts{},
			})
		}

		return c
	}
}

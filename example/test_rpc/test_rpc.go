package test_rpc

import (
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/rpc"
	rpc_channel "github.com/sniperHW/kendynet/rpc/channel"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/pb"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/tcp"
	codec "github.com/sniperHW/kendynet/codec/stream_socket"
	"reflect"
	"fmt"
	"time"
)


type TcpStreamChannel struct {
	session    kendynet.StreamSession
	name       string
}

func NewTcpStreamChannel(sess kendynet.StreamSession) *TcpStreamChannel {
	r := &TcpStreamChannel{session:sess}
	r.name = sess.RemoteAddr().String() + "<->" + sess.LocalAddr().String() 
	return r
}

func(this *TcpStreamChannel) SendRPCRequest(message interface {}) error {
	return this.session.Send(message)
}

func(this *TcpStreamChannel) SendRPCResponse(message interface {}) error {
	return this.session.Send(message)
}

func(this *TcpStreamChannel) Close(reason string) {
	this.session.Close(reason,0)
}

func(this *TcpStreamChannel) Name() string {
	return this.name
}

type TestEncoder struct {
}

func (this *TestEncoder) Encode(message rpc.RPCMessage) (interface{},error) {
	if message.Type() == rpc.RPC_REQUEST {
		req := message.(*rpc.RPCRequest)
		request := &testproto.RPCRequest{}
		request.Seq = proto.Uint64(req.Seq)
		request.Method = proto.String(req.Method)
		request.NeedResp = proto.Bool(req.NeedResp)
		if req.Arg != nil {
			buff,err := pb.Encode(req.Arg,1000)
			if err != nil {
				fmt.Printf("encode error: %s\n",err.Error())
				return nil,err
			}
			request.Arg = buff.Bytes()
		}
		return request,nil
	} else if message.Type() == rpc.RPC_PING {
		req := message.(*rpc.Ping)
		request := &testproto.RPCPing{}
		request.Seq = proto.Uint64(req.Seq)	
		request.Timestamp = proto.Int64(req.TimeStamp)	
		return request,nil
	} else if message.Type() == rpc.RPC_PONG {
		resp := message.(*rpc.Pong)
		response := &testproto.RPCPong{}
		response.Seq = proto.Uint64(resp.Seq)	
		response.Timestamp = proto.Int64(resp.TimeStamp)	
		return response,nil
	} else {
		resp := message.(*rpc.RPCResponse)
		response := &testproto.RPCResponse{}
		response.Seq = proto.Uint64(resp.Seq)
		if resp.Err != nil {
			response.Err = proto.String(resp.Err.Error())
		}
		if resp.Ret != nil {
			buff,err := pb.Encode(resp.Ret,1000)
			if err != nil {
				fmt.Printf("encode error: %s\n",err.Error())
				return nil,err
			}
			response.Ret = buff.Bytes()
		}
		return response,nil
	}
}

type TestDecoder struct {
}

func (this *TestDecoder) Decode(o interface{}) (rpc.RPCMessage,error) {	
	switch o.(type) {
		case *testproto.RPCRequest:
			req := o.(*testproto.RPCRequest)
			request := &rpc.RPCRequest{}
			request.Seq = req.GetSeq()
			request.Method = req.GetMethod()
			request.NeedResp = req.GetNeedResp()
			if len(req.Arg) > 0 {
				var err error
				request.Arg,_,err = pb.Decode(req.Arg,0,(uint64)(len(req.Arg)),1000)
				if err != nil {
					return nil,err
				}
			}

			return request,nil
		case *testproto.RPCResponse:
			resp := o.(*testproto.RPCResponse)
			response := &rpc.RPCResponse{}
			response.Seq = resp.GetSeq()
			if resp.Err != nil {
				response.Err = fmt.Errorf(resp.GetErr())
			}
			if len(resp.Ret) > 0 {
				var err error
				response.Ret,_,err = pb.Decode(resp.Ret,0,(uint64)(len(resp.Ret)),1000)
				if err != nil {
					return nil,err
				}
			}
			return response,nil
		case *testproto.RPCPing:
			req := o.(*testproto.RPCPing)
			request := &rpc.Ping{}
			request.Seq = req.GetSeq()
			request.TimeStamp = req.GetTimestamp()
			return request,nil
		case *testproto.RPCPong:
			resp := o.(*testproto.RPCPong)
			response := &rpc.Ping{}
			response.Seq = resp.GetSeq()
			response.TimeStamp = resp.GetTimestamp()
			return response,nil
		default:
			return nil,fmt.Errorf("invaild obj type:%s",reflect.TypeOf(o).String())
	}	
}

type RPCServer struct {
	server     *rpc.RPCServer
	listener    kendynet.Listener
}

func NewRPCServer() *RPCServer {
	r := &RPCServer{}
	r.server,_ = rpc.NewRPCServer(&TestDecoder{},&TestEncoder{})
	return r
}

func (this *RPCServer) RegisterMethod(name string,method rpc.RPCMethodHandler) {
	this.server.RegisterMethod(name,method)
}

func (this *RPCServer) Serve(service string) error {
	var err error
	this.listener,err = tcp.NewListener("tcp",service)
	if err != nil {
		return err
	}

	err = this.listener.Start(func(session kendynet.StreamSession){
		channel := NewTcpStreamChannel(session)
		session.SetEncoder(codec.NewPbEncoder(4096))
		session.SetReceiver(codec.NewPBReceiver(4096))
		session.SetEventCallBack(func (event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				channel.Close(event.Data.(error).Error())
			} else {
				this.server.OnRPCMessage(channel,event.Data)	
			}
		})
		session.Start()
	})
	return err
}

/*
*  round robin selector
*/

type testSelector struct {
	idx int
}

func (this *testSelector) Select(channels []rpc_channel.RPCChannel) rpc_channel.RPCChannel {
	l := len(channels)
	if l == 0 {
		return nil
	}
	c := channels[this.idx]
	this.idx = (this.idx + 1) % l
	return c
}


type Caller struct {
	client     *rpc.RPCClient
	method      string
	selector    rpc.ChannelSelector
}

func NewCaller(method string) *Caller {
	c := &Caller{}
	c.method = method
	c.client,_ = rpc.NewRPCClient(&TestDecoder{},&TestEncoder{},&rpc.Option{Timeout:5000*time.Millisecond,PingInterval:5000*time.Millisecond,PingTimeout:5000*time.Millisecond})
	c.selector = &testSelector{}
	return c
}

func (this *Caller) Dial(service string,timeout time.Duration) error {
	connector,err := tcp.NewConnector("tcp",service)
	session,_,err := connector.Dial(timeout)
	if err != nil {
		return err
	} 
	channel := NewTcpStreamChannel(session)
	this.client.AddMethod(channel,this.method)
	session.SetEncoder(codec.NewPbEncoder(4096))
	session.SetReceiver(codec.NewPBReceiver(4096))
	session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
		this.client.OnChannelClose(channel,fmt.Errorf(reason))
	})
	session.SetEventCallBack(func (event *kendynet.Event) {
		if event.EventType == kendynet.EventTypeError {
			channel.Close(event.Data.(error).Error())
		} else {
			this.client.OnRPCMessage(channel,event.Data)	
		}
	})
	session.Start()
	return nil
}

func (this *Caller) Call(arg interface{},cb rpc.RPCResponseHandler) error {
	return this.client.Call(this.selector,this.method,arg,cb)
}


package test_rpc

import (
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet/rpc"
	"github.com/sniperHW/kendynet/example/testproto"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/socket/stream_socket/tcp"
	codec "github.com/sniperHW/kendynet/example/codec/stream_socket"
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

func(this *TcpStreamChannel) SendRequest(message interface {}) error {
	return this.session.Send(message)
}

func(this *TcpStreamChannel) SendResponse(message interface {}) error {
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
		session.Start(func (event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				channel.Close(event.Data.(error).Error())
			} else {
				this.server.OnRPCMessage(channel,event.Data)
			}
		})
	})
	return err
}


type Caller struct {
	client     *rpc.RPCClient
}

func NewCaller() *Caller {
	return &Caller{}
}

func (this *Caller) Dial(service string,timeout time.Duration) error {
	connector,err := tcp.NewConnector("tcp",service)
	session,err := connector.Dial(timeout)
	if err != nil {
		return err
	} 
	channel := NewTcpStreamChannel(session)
	this.client,_ = rpc.NewClient(channel,&TestDecoder{},&TestEncoder{})
	session.SetEncoder(codec.NewPbEncoder(4096))
	session.SetReceiver(codec.NewPBReceiver(4096))
	session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
		this.client.OnChannelClose(fmt.Errorf(reason))
		fmt.Printf("channel close:%s\n",reason)
	})
	session.Start(func (event *kendynet.Event) {
		if event.EventType == kendynet.EventTypeError {
			channel.Close(event.Data.(error).Error())
		} else {
			this.client.OnRPCMessage(event.Data)	
		}
	})
	return nil
}

func (this *Caller) AsynCall(method string,arg interface{},cb rpc.RPCResponseHandler) error {
	return this.client.AsynCall(method,arg,0,cb)
}

func (this *Caller) SyncCall(method string,arg interface{}) (interface{},error) {
	return this.client.SyncCall(method,arg,1)//uint32(time.Millisecond*5))
}


/*
type Caller struct {
	client     *rpc.RPCClient
	method      string
	selector    rpc.ChannelSelector
}

func NewCaller(method string) *Caller {
	c := &Caller{}
	c.method = method
	option := &rpc.Option{}
	option.Timeout = 5000*time.Millisecond
	c.client,_ = rpc.NewRPCClient(&TestDecoder{},&TestEncoder{},option)
	c.selector = &testSelector{}
	return c
}

func (this *Caller) Dial(service string,timeout time.Duration) error {
	connector,err := tcp.NewConnector("tcp",service)
	session,err := connector.Dial(timeout)
	if err != nil {
		return err
	} 
	channel := NewTcpStreamChannel(session)
	this.client.AddMethod(channel,this.method)
	session.SetEncoder(codec.NewPbEncoder(4096))
	session.SetReceiver(codec.NewPBReceiver(4096))
	session.SetCloseCallBack(func (sess kendynet.StreamSession, reason string) {
		this.client.OnChannelClose(channel,fmt.Errorf(reason))
		fmt.Printf("channel close:%s\n",reason)
	})
	session.Start(func (event *kendynet.Event) {
		if event.EventType == kendynet.EventTypeError {
			channel.Close(event.Data.(error).Error())
		} else {
			this.client.OnRPCMessage(channel,event.Data)	
		}
	})
	return nil
}

func (this *Caller) Call(arg interface{},cb rpc.RPCResponseHandler) error {
	return this.client.Call(this.selector,this.method,arg,cb)
}
*/

func init() {
	pb.Register(&testproto.Hello{},1)
	pb.Register(&testproto.World{},2)
	pb.Register(&testproto.RPCResponse{},3)
	pb.Register(&testproto.RPCRequest{},4)
}


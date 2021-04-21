package rpc

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go tool cover -html=coverage.out

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/event"
	codec "github.com/sniperHW/kendynet/example/codec"
	"github.com/sniperHW/kendynet/example/pb"
	"github.com/sniperHW/kendynet/example/testproto"
	connector "github.com/sniperHW/kendynet/socket/connector/tcp"
	listener "github.com/sniperHW/kendynet/socket/listener/tcp"
	//"github.com/sniperHW/kendynet/util"
	"github.com/stretchr/testify/assert"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

type TcpStreamChannel struct {
	session kendynet.StreamSession
	name    string
}

func NewTcpStreamChannel(sess kendynet.StreamSession) *TcpStreamChannel {
	r := &TcpStreamChannel{session: sess}
	r.name = sess.RemoteAddr().String() + "<->" + sess.LocalAddr().String()
	return r
}

func (this *TcpStreamChannel) SendRequest(message interface{}) error {
	return this.session.Send(message)
}

func (this *TcpStreamChannel) SendResponse(message interface{}) error {
	return this.session.Send(message)
}

func (this *TcpStreamChannel) Name() string {
	return this.name
}

func (this *TcpStreamChannel) UID() uint64 {
	return uint64(uintptr(unsafe.Pointer(this)))
}

func (this *TcpStreamChannel) GetSession() kendynet.StreamSession {
	return this.session
}

type ErrorEncoder struct {
}

func (this *ErrorEncoder) Encode(message RPCMessage) (interface{}, error) {
	return nil, fmt.Errorf("error")
}

type TestEncoder struct {
}

func (this *TestEncoder) Encode(message RPCMessage) (interface{}, error) {
	if message.Type() == RPC_REQUEST {
		req := message.(*RPCRequest)
		req.GetSeq()
		request := &testproto.RPCRequest{
			Seq:      proto.Uint64(req.Seq),
			Method:   proto.String(req.Method),
			NeedResp: proto.Bool(req.NeedResp),
		}
		if req.Arg != nil {
			b, err := pb.Encode(req.Arg, nil, 1000)
			if err != nil {
				fmt.Printf("encode error: %s\n", err.Error())
				return nil, err
			}
			request.Arg = b.Bytes()
		}
		return request, nil
	} else {
		resp := message.(*RPCResponse)
		response := &testproto.RPCResponse{Seq: proto.Uint64(resp.Seq)}
		if resp.Err != nil {
			response.Err = proto.String(resp.Err.Error())
		}
		if resp.Ret != nil {
			b, err := pb.Encode(resp.Ret, nil, 1000)
			if err != nil {
				fmt.Printf("encode error: %s\n", err.Error())
				return nil, err
			}
			response.Ret = b.Bytes()
		}
		return response, nil
	}
}

type ErrorDecoder struct {
}

func (this *ErrorDecoder) Decode(o interface{}) (RPCMessage, error) {
	return nil, fmt.Errorf("error")
}

type TestDecoder struct {
}

func (this *TestDecoder) Decode(o interface{}) (RPCMessage, error) {
	switch o.(type) {
	case *testproto.RPCRequest:
		req := o.(*testproto.RPCRequest)
		request := &RPCRequest{
			Seq:      req.GetSeq(),
			Method:   req.GetMethod(),
			NeedResp: req.GetNeedResp(),
		}
		if len(req.Arg) > 0 {
			var err error
			request.Arg, _, err = pb.Decode(req.Arg, 0, len(req.Arg), 1000)
			if err != nil {
				return nil, err
			}
		}

		return request, nil
	case *testproto.RPCResponse:
		resp := o.(*testproto.RPCResponse)
		response := &RPCResponse{Seq: resp.GetSeq()}
		if resp.Err != nil {
			response.Err = fmt.Errorf(resp.GetErr())
		}
		if len(resp.Ret) > 0 {
			var err error
			response.Ret, _, err = pb.Decode(resp.Ret, 0, len(resp.Ret), 1000)
			if err != nil {
				return nil, err
			}
		}
		return response, nil
	default:
		return nil, fmt.Errorf("invaild obj type:%s", reflect.TypeOf(o).String())
	}
}

var errHalt error = fmt.Errorf("halt")

type TestRPCServer struct {
	server   *RPCServer
	listener *listener.Listener
	halt     atomic.Value
}

func NewTestRPCServer(decoder RPCMessageDecoder, encoder RPCMessageEncoder) *TestRPCServer {
	return &TestRPCServer{
		server: NewRPCServer(decoder, encoder),
	}
}

func (this *TestRPCServer) RegisterMethod(name string, method RPCMethodHandler) error {
	return this.server.RegisterMethod(name, method)
}

func (this *TestRPCServer) Serve(service string) error {
	var err error
	this.listener, err = listener.New("tcp", service)
	if err != nil {
		return err
	}

	err = this.listener.Serve(func(session kendynet.StreamSession) {
		channel := NewTcpStreamChannel(session)
		session.SetEncoder(codec.NewPbEncoder(65535)).SetInBoundProcessor(codec.NewPBReceiver(65535)).SetRecvTimeout(5 * time.Second)
		session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
			if this.halt.Load() != nil {
				this.server.OnServiceStop(channel, msg, errHalt)
			} else {
				this.server.OnRPCMessage(channel, msg)
			}
		})
	})
	return err
}

func (this *TestRPCServer) Stop() {
	this.listener.Close()
}

type Caller struct {
	client  *RPCClient
	channel RPCChannel
	decoder RPCMessageDecoder
	encoder RPCMessageEncoder
}

func NewCaller(decoder RPCMessageDecoder, encoder RPCMessageEncoder) *Caller {
	c := &Caller{
		decoder: decoder,
		encoder: encoder,
	}
	return c
}

func (this *Caller) Dial(service string, timeout time.Duration, queue *event.EventQueue) error {
	connector, err := connector.New("tcp", service)
	session, err := connector.Dial(timeout)
	if err != nil {
		return err
	}
	this.channel = NewTcpStreamChannel(session)
	this.client = NewClient(this.decoder, this.encoder)
	session.SetEncoder(codec.NewPbEncoder(65535)).SetInBoundProcessor(codec.NewPBReceiver(65535)).SetRecvTimeout(5 * time.Second)
	session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
		fmt.Println("channel close", reason)
		this.client.OnChannelDisconnect(this.channel)
	}).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
		this.client.OnRPCMessage(msg)
	})
	return nil
}

func (this *Caller) Post(method string, arg interface{}) error {
	return this.client.Post(this.channel, method, arg)
}

func (this *Caller) AsynCall(method string, arg interface{}, timeout time.Duration, cb RPCResponseHandler) error {
	return this.client.AsynCall(this.channel, method, arg, timeout, cb)
}

func (this *Caller) Call(method string, arg interface{}, timeout time.Duration) (interface{}, error) {
	return this.client.Call(this.channel, method, arg, timeout)
}

func init() {
	pb.Register(&testproto.Hello{}, 1)
	pb.Register(&testproto.World{}, 2)
	pb.Register(&testproto.RPCResponse{}, 3)
	pb.Register(&testproto.RPCRequest{}, 4)
}

func TestRPC2(t *testing.T) {
	fmt.Println("0000000000000")
	{

		server := NewTestRPCServer(&ErrorDecoder{}, &ErrorEncoder{})

		server.RegisterMethod("hello", func(replyer *RPCReplyer, arg interface{}) {
			world := &testproto.World{World: proto.String("world")}
			replyer.Reply(world, nil)
		})

		go server.Serve("localhost:8111")

		caller := NewCaller(&TestDecoder{}, &TestEncoder{})
		caller.Dial("localhost:8111", 10*time.Second, nil)
		caller.Post("hello", &testproto.Hello{Hello: proto.String("hello")})
		time.Sleep(time.Second)
		server.halt.Store(true)
		caller.Post("hello", &testproto.Hello{Hello: proto.String("hello")})
		time.Sleep(time.Second)

	}

	fmt.Println("11111111111")

	{

		server := NewTestRPCServer(&TestDecoder{}, &ErrorEncoder{})

		server.RegisterMethod("hello", func(replyer *RPCReplyer, arg interface{}) {

			str := arg.(*testproto.Hello).GetHello()
			fmt.Println(str)
			if str == "hello2" {
				replyer.GetChannel().(*TcpStreamChannel).session.Close(nil, 0)
			}
			world := &testproto.World{World: proto.String("world")}
			replyer.Reply(world, nil)
		})

		go server.Serve("localhost:8112")

		caller := NewCaller(&TestDecoder{}, &TestEncoder{})
		caller.Dial("localhost:8112", 10*time.Second, nil)

		caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second, func(r interface{}, err error) {
		})

		time.Sleep(time.Second)

		server.halt.Store(true)

		caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second, func(r interface{}, err error) {
		})

		time.Sleep(time.Second)

		server.halt.Store(false)

	}

	fmt.Println("222222222222222")

	{

		server := NewTestRPCServer(&TestDecoder{}, &TestEncoder{})

		server.RegisterMethod("hello", func(replyer *RPCReplyer, arg interface{}) {
			replyer.GetChannel().(*TcpStreamChannel).session.Close(nil, 0)
			world := &testproto.World{World: proto.String("world")}
			replyer.Reply(world, nil)
		})

		go server.Serve("localhost:8113")

		caller := NewCaller(&TestDecoder{}, &TestEncoder{})
		caller.Dial("localhost:8113", 10*time.Second, nil)

		caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second, func(r interface{}, err error) {
		})

		time.Sleep(time.Second)

	}

}

func TestRPC(t *testing.T) {

	assert.Nil(t, NewClient(nil, nil))

	assert.Nil(t, NewRPCServer(nil, nil))

	server := NewTestRPCServer(&TestDecoder{}, &TestEncoder{})

	assert.Nil(t, server.RegisterMethod("hello", func(replyer *RPCReplyer, arg interface{}) {}))
	assert.NotNil(t, server.RegisterMethod("hello", func(replyer *RPCReplyer, arg interface{}) {}))
	server.server.UnRegisterMethod("hello")

	server.RegisterMethod("", nil)
	server.RegisterMethod("bad", nil)

	server.RegisterMethod("panic", func(replyer *RPCReplyer, arg interface{}) {
		panic("panic")
	})

	//注册服务
	assert.Nil(t, server.RegisterMethod("hello", func(replyer *RPCReplyer, arg interface{}) {
		world := &testproto.World{World: proto.String("world")}
		str := arg.(*testproto.Hello).GetHello()
		if str == "testtimeout" {
			time.Sleep(time.Second * 3)
		} else if str == "testdrop" {
			replyer.DropResponse()
			return
		}
		replyer.Reply(world, nil)
	}))

	go server.Serve("localhost:8110")

	server.server.PendingCount()

	{
		caller := NewCaller(&TestDecoder{}, &TestEncoder{})

		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))

		caller.client.OnRPCMessage("hello")

		assert.Nil(t, caller.Post("hello", &testproto.Hello{Hello: proto.String("hello")}))

		{
			caller.Call("panic", &testproto.Hello{Hello: proto.String("hello")}, time.Second*2)
		}

		{
			r, err := caller.Call("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second*2)
			assert.Nil(t, err)
			assert.Equal(t, r.(*testproto.World).GetWorld(), "world")
		}

		{
			_, err := caller.Call("world", &testproto.Hello{Hello: proto.String("hello")}, time.Second*2)
			assert.Equal(t, err.Error(), "invaild method:world")
		}

		{
			_, err := caller.Call("hello", &testproto.Hello{Hello: proto.String("testtimeout")}, time.Second*2)
			assert.Equal(t, err, ErrCallTimeout)
		}

		server.server.SetErrorCodeOnMissingMethod(fmt.Errorf("invaild method:world"))

		{
			fmt.Println("5 begin", time.Now())
			_, err := caller.Call("world", &testproto.Hello{Hello: proto.String("hello")}, time.Second*2)
			assert.Equal(t, err.Error(), "invaild method:world")
		}

		{
			_, err := caller.Call("hello", &testproto.Hello{Hello: proto.String("testdrop")}, time.Second*2)
			assert.Equal(t, err, ErrCallTimeout)
		}

		time.Sleep(time.Second * 4)

		{
			caller.channel.(*TcpStreamChannel).session.Close(nil, 0)
			{
				err := caller.Post("hello", &testproto.Hello{Hello: proto.String("hello")})
				assert.Equal(t, err, kendynet.ErrSocketClose)
			}

			{
				_, err := caller.Call("hello", &testproto.Hello{Hello: proto.String("testtimeout")}, time.Second*2)
				assert.Equal(t, err, kendynet.ErrSocketClose)
			}
		}

	}

	{
		//asyncall
		caller := NewCaller(&TestDecoder{}, &TestEncoder{})
		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))
		ok := make(chan struct{})

		caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second, func(r interface{}, err error) {
			assert.Nil(t, err)
			assert.Equal(t, r.(*testproto.World).GetWorld(), "world")
			close(ok)
		})

		<-ok

	}

	{

		caller := NewCaller(&ErrorDecoder{}, &ErrorEncoder{})
		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))

		assert.NotNil(t, caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second, func(r interface{}, err error) {

		}))

		assert.NotNil(t, caller.Post("hello", &testproto.Hello{Hello: proto.String("hello")}))

		assert.NotNil(t, caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second, nil))
	}

	{
		//test channel disconnected
		caller := NewCaller(&TestDecoder{}, &TestEncoder{})

		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))

		ok := make(chan struct{})

		caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("testtimeout")}, 2*time.Second, func(r interface{}, err error) {
			assert.Equal(t, ErrChannelDisconnected, err)
			close(ok)
		})

		time.Sleep(time.Second)

		caller.channel.(*TcpStreamChannel).session.Close(nil, 0)

		<-ok

		time.Sleep(time.Second)

	}

	{

		caller := NewCaller(&TestDecoder{}, &TestEncoder{})

		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))

		server.halt.Store(true)

		_, err := caller.Call("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second)
		assert.Equal(t, err, errHalt)

	}

	time.Sleep(time.Second * 3)

	assert.Equal(t, int32(0), server.server.PendingCount())

}

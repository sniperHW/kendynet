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
	"github.com/stretchr/testify/assert"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
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

func (this *TcpStreamChannel) GetSession() kendynet.StreamSession {
	return this.session
}

type TestEncoder struct {
}

func (this *TestEncoder) Encode(message RPCMessage) (interface{}, error) {
	if message.Type() == RPC_REQUEST {
		req := message.(*RPCRequest)
		request := &testproto.RPCRequest{
			Seq:      proto.Uint64(req.Seq),
			Method:   proto.String(req.Method),
			NeedResp: proto.Bool(req.NeedResp),
		}
		if req.Arg != nil {
			buff, err := pb.Encode(req.Arg, 1000)
			if err != nil {
				fmt.Printf("encode error: %s\n", err.Error())
				return nil, err
			}
			request.Arg = buff.Bytes()
		}
		return request, nil
	} else {
		resp := message.(*RPCResponse)
		response := &testproto.RPCResponse{Seq: proto.Uint64(resp.Seq)}
		if resp.Err != nil {
			response.Err = proto.String(resp.Err.Error())
		}
		if resp.Ret != nil {
			buff, err := pb.Encode(resp.Ret, 1000)
			if err != nil {
				fmt.Printf("encode error: %s\n", err.Error())
				return nil, err
			}
			response.Ret = buff.Bytes()
		}
		return response, nil
	}
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
			request.Arg, _, err = pb.Decode(req.Arg, 0, (uint64)(len(req.Arg)), 1000)
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
			response.Ret, _, err = pb.Decode(resp.Ret, 0, (uint64)(len(resp.Ret)), 1000)
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

func NewTestRPCServer() *TestRPCServer {
	return &TestRPCServer{
		server: NewRPCServer(&TestDecoder{}, &TestEncoder{}),
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
		session.SetEncoder(codec.NewPbEncoder(65535))
		session.SetReceiver(codec.NewPBReceiver(65535))
		session.SetRecvTimeout(5 * time.Second)
		session.Start(func(event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				session.Close(event.Data.(error).Error(), 0)
			} else {
				if this.halt.Load() != nil {
					req, _ := this.server.decoder.Decode(event.Data)
					this.server.DirectReplyError(channel, req.(*RPCRequest), errHalt)
				} else {
					this.server.OnRPCMessage(channel, event.Data)
				}
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
}

func NewCaller() *Caller {
	return &Caller{}
}

func (this *Caller) Dial(service string, timeout time.Duration, queue *event.EventQueue) error {
	connector, err := connector.New("tcp", service)
	session, err := connector.Dial(timeout)
	if err != nil {
		return err
	}
	this.channel = NewTcpStreamChannel(session)
	this.client = NewClient(&TestDecoder{}, &TestEncoder{}, queue)
	session.SetEncoder(codec.NewPbEncoder(65535))
	session.SetReceiver(codec.NewPBReceiver(65535))
	session.SetRecvTimeout(5 * time.Second)
	session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
		fmt.Printf("channel close:%s\n", reason)
	})
	session.Start(func(event *kendynet.Event) {
		if event.EventType == kendynet.EventTypeError {
			session.Close(event.Data.(error).Error(), 0)
		} else {
			this.client.OnRPCMessage(event.Data)
		}
	})
	return nil
}

func (this *Caller) Post(method string, arg interface{}) error {
	return this.client.Post(this.channel, method, arg)
}

func (this *Caller) AsynCall(method string, arg interface{}, timeout time.Duration, cb RPCResponseHandler) {
	this.client.AsynCall(this.channel, method, arg, timeout, cb)
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

func TestRPC(t *testing.T) {

	assert.Nil(t, NewClient(nil, nil, nil))

	assert.Nil(t, NewRPCServer(nil, nil))

	server := NewTestRPCServer()

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
			time.Sleep(time.Second * 2)
		} else if str == "testdrop" {
			replyer.DropResponse()
			return
		}
		replyer.Reply(world, nil)
	}))

	go server.Serve("localhost:8110")

	server.server.PendingCount()

	{
		caller := NewCaller()

		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))

		caller.client.OnRPCMessage("hello")

		caller.client.PendingCount()

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

		server.server.SetOnMissingMethod(func(method string, replyer *RPCReplyer) {
			fmt.Println("OnMissingMethod", method, time.Now())
			replyer.Reply(nil, fmt.Errorf("invaild method:%s", method))
		})

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
			caller.channel.(*TcpStreamChannel).session.Close("none", 0)
			{
				err := caller.Post("hello", &testproto.Hello{Hello: proto.String("hello")})
				assert.Equal(t, err, kendynet.ErrSocketClose)
			}

			{
				_, err := caller.Call("hello", &testproto.Hello{Hello: proto.String("testtimeout")}, time.Second*2)
				assert.Equal(t, err, kendynet.ErrSocketClose)
			}
		}

		assert.Equal(t, int32(0), caller.client.PendingCount())

	}

	{
		//asyncall
		caller := NewCaller()
		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))
		ok := make(chan struct{})

		caller.AsynCall("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second, func(r interface{}, err error) {
			assert.Nil(t, err)
			assert.Equal(t, r.(*testproto.World).GetWorld(), "world")
			close(ok)
		})

		<-ok

		assert.Equal(t, int32(0), caller.client.PendingCount())
	}

	{
		//with eventqueue
		queue := event.NewEventQueue()
		go queue.Run()

		caller := NewCaller()
		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, queue))
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

		time.Sleep(time.Second * 4)

		queue.Close()

		assert.Equal(t, int32(0), caller.client.PendingCount())
	}

	{

		caller := NewCaller()

		assert.Nil(t, caller.Dial("localhost:8110", 10*time.Second, nil))

		server.halt.Store(true)

		_, err := caller.Call("hello", &testproto.Hello{Hello: proto.String("hello")}, time.Second)
		assert.Equal(t, err, errHalt)

		assert.Equal(t, int32(0), caller.client.PendingCount())
	}

	assert.Equal(t, int32(0), server.server.PendingCount())

}

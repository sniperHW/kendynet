package rpc

const (
	RPC_REQUEST  = 1
	RPC_RESPONSE = 2
	RPC_PING     = 3
	RPC_PONG     = 4
)

type RPCMessage interface {
	Type()   byte
	GetSeq() uint64
}

type RPCRequest struct {
	Seq      uint64
	Method   string
	Arg      interface{}
	NeedResp bool 
}

type RPCResponse struct {
	Seq    uint64
	Err    error
	Ret    interface{}
}

type Ping struct {
	Seq      uint64
	TimeStamp int64   //客户端的时间戳
}

type Pong struct {
	Seq       uint64
	TimeStamp int64   //使用Ping受到的时间戳，客户端可用于估算RTT
}

func (this *RPCRequest) Type() byte {
	return RPC_REQUEST
}

func (this *RPCResponse) Type() byte {
	return RPC_RESPONSE
}

func (this *Ping) Type() byte {
	return RPC_PING
}

func (this *Pong) Type() byte {
	return RPC_PONG
}

func (this *RPCRequest) GetSeq() uint64 {
	return this.Seq
}

func (this *RPCResponse) GetSeq() uint64 {
	return this.Seq
}

func (this *Ping) GetSeq() uint64 {
	return this.Seq
}

func (this *Pong) GetSeq() uint64 {
	return this.Seq
}

type RPCMessageEncoder interface {
	Encode(RPCMessage) (interface{},error)
}

type RPCMessageDecoder interface {
	Decode(interface{}) (RPCMessage,error)	
}
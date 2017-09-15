package channel

/*
 *  rpc通道，实现了RPCChannel的类型都可用于发送rpc消息
*/

type RPCChannel interface {
	SendRPCRequest(interface {}) error
	SendRPCResponse(interface {}) error
	Name() string
}
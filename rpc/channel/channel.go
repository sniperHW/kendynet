package channel

/*
 *  rpc通道，实现了RPCChannel的类型都可用于发送rpc消息
*/

type RPCChannel interface {
	SendRPCMessage(interface {}) error
	Name() string
}
package channel

import (
//	"time"
)

/*
 *  rpc通道，实现了RPCChannel的类型都可用于发送rpc消息
*/

type RPCChannel interface {
	SendRPCRequest(interface {}) error               //发送RPC请求
	SendRPCResponse(interface {}) error              //发送RPC回应
	Close(string)
	Name() string
}
package stream_socket

import (
	"github.com/sniperHW/kendynet"
)

/*
*   发送缓冲处理器接口
*   如果设置了发送缓冲处理器，在将缓冲队列中的数据包发送出去之前，首先将队列交给SendBuffProcessor处理
*   SendBuffProcessor会返回一组处理后的缓冲队列
*   例如一个SendBuffProcessor的实现是将输入队列中的数据包合并成一个包，并执行压缩或加密。
*/
type SendBuffProcessor interface {
	Process([] kendynet.Message) [] kendynet.Message
}
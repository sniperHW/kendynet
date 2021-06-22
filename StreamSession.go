package kendynet

import (
	"github.com/sniperHW/kendynet/buffer"
	"net"
	"time"
)

const (
	EventTypeMessage = 1
	EventTypeError   = 2
)

type InBoundProcessor interface {
	Unpack() (interface{}, error)
}

type EnCoder interface {
	EnCode(o interface{}, b *buffer.Buffer) error
}

type StreamSession interface {

	/*
	 * 将o投递到发送队列
	 * 如果发送队列满直接返回busy
	 */

	Send(o interface{}) error

	/*
	 * 将o投递到发送队列
	 * timeout <= 0,如果发送队列满，永久阻塞
	 * timeout > 0,如果发送队列满阻塞到timeout
	 */
	SendWithTimeout(o interface{}, timeout time.Duration) error

	/*
	 * 阻塞式直接发送[]byte,不应与Send混用
	 */
	DirectSend([]byte, ...time.Duration) (int, error)

	/*
		关闭会话,如果会话中还有待发送的数据且timeout > 0
		将尝试将数据发送完毕后关闭，如果数据未能完成发送则等到timeout秒之后也会被关闭。

		无论何种情况，调用Close之后SendXXX操作都将返回错误
	*/
	Close(reason error, timeout time.Duration)

	ShutdownRead()

	ShutdownWrite()

	IsClosed() bool

	/*
		设置关闭回调，当session被关闭时回调
		其中reason参数表明关闭原因由Close函数传入
		需要注意，回调可能在接收或发送goroutine中调用，如回调函数涉及数据竞争，需要自己加锁保护
	*/
	SetCloseCallBack(cb func(StreamSession, error)) StreamSession

	SetErrorCallBack(cb func(StreamSession, error)) StreamSession

	SetInBoundProcessor(InBoundProcessor) StreamSession

	/*
	 *  设置消息序列化器，用于将一个对象序列化成Message对象，
	 *  如果没有设置Send和PostSend将返回错误(只能调用SendMessage,PostSendMessage发送Message)
	 */
	SetEncoder(encoder EnCoder) StreamSession

	BeginRecv(func(StreamSession, interface{})) error

	LocalAddr() net.Addr

	RemoteAddr() net.Addr

	SetUserData(ud interface{}) StreamSession

	GetUserData() interface{}

	GetUnderConn() interface{}

	SetRecvTimeout(time.Duration) StreamSession

	SetSendTimeout(time.Duration) StreamSession

	/*
	 *   设置异步发送队列大小
	 */
	SetSendQueueSize(int) StreamSession
}

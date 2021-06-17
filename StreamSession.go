/*
面向流的会话接口
*/

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
	 * timeout:如果没有设置timeout,当发送队列满不将o投递到队列，直接返回busy
	 * timeout <= 0,如果发送队列满，永久阻塞
	 * timeout > 0,如果发送队列满阻塞到timeout
	 */

	Send(o interface{}, timeout ...time.Duration) error

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
	/*
	 *   获取会话的本端地址
	 */
	LocalAddr() net.Addr

	/*
	 *   获取会话的对端地址
	 */
	RemoteAddr() net.Addr

	/*
	 *   设置用户数据
	 */
	SetUserData(ud interface{}) StreamSession

	/*
	 *   获取用户数据
	 */
	GetUserData() interface{}

	GetUnderConn() interface{}

	SetRecvTimeout(time.Duration) StreamSession

	SetSendTimeout(time.Duration) StreamSession

	/*
	 *   设置异步发送队列大小,必须在调用Start前设置
	 */
	SetSendQueueSize(int) StreamSession
}

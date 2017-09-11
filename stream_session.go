/*
面向流的会话接口
*/

package kendynet

import (
	   "net"
       "fmt"
       "time"
)

var (
    ErrSocketClose       = fmt.Errorf("socket close")
    ErrSendTimeout       = fmt.Errorf("send timeout")
    ErrStarted           = fmt.Errorf("already started")
    ErrInvaildBuff       = fmt.Errorf("buff is nil")
    ErrNoOnPacket        = fmt.Errorf("onPacket == nil")
    ErrNoReceiver        = fmt.Errorf("receiver == nil")
    ErrInvaildObject     = fmt.Errorf("object == nil")
    ErrInvaildEncoder    = fmt.Errorf("encoder == nil")
    ErrInvaildWSMessage  = fmt.Errorf("invaild websocket message")
    ErrWSPeerClose       = fmt.Errorf("receive websocket peer close")
    ErrWSInvaildUpgrader = fmt.Errorf("upgrader == nil")
)

const (
    EventTypeMessage = 1
    EventTypeError   = 2
)

const (
    DefaultSendTimeout = 10
)

type Event struct {
    EventType int16
    Session StreamSession
    Data    interface {}    
}

type StreamSession interface {
	/* 
		发送一个对象，使用encoder将对象编码成一个Message调用SendMessage
	*/
	Send(o interface{}) error
	
	/* 
		直接发送Message
	*/
	SendMessage(msg Message) error
	
	/* 
		关闭会话,如果会话中还有待发送的数据且timeout非0
		将尝试将数据发送完毕后关闭，如果数据未能完成发送则等到timeout秒之后也会被关闭。

		无论何种情况，调用Close之后SendXXX操作都将返回错误
	*/
    Close(reason string, timeout time.Duration) error
    
    /*
    	设置关闭回调，当session被关闭时回调
    	其中reason参数表明关闭原因由Close函数传入
    	需要注意，回调可能在接收或发送goroutine中调用，如回调函数涉及数据竞争，需要自己加锁保护
    */
    SetCloseCallBack(cb func (StreamSession,string))

    /*
    *   设置事件回调
    *   需要注意，回调可能在接收或发送goroutine中调用，如回调函数涉及数据竞争，需要自己加锁保护
    */
    SetEventCallBack(cb func (*Event))

    /*
    *   设置接收解包器
    */
    SetReceiver(r Receiver)

    SetEncoder(encoder EnCoder)

    /*
    *   设置接收超时(如果出现超时调用SetPacketCallBack中传入的回调函数，传递合适的错误)
    */ 
    SetReceiveTimeout(timeout time.Duration)

    /*
    *	设置发送超时(如果出现超时调用SetPacketCallBack中传入的回调函数，传递合适的错误)
    */
    SetSendTimeout(timeout time.Duration)

    /*
    *	获取待发送数据的大小
    */
    //GetSendBuffSize() uint32

    /*
    *   启动会话处理
    */
    Start() error

    /*
    *   暂停会话处理的接收处理
    */
    //PauseReceive() error

    /*
    *   恢复会话处理的接收处理
    */    
    //ResumeReceive() error

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
    SetUserData(ud interface{})

    /*
    *   获取用户数据
    */ 
    GetUserData() interface{}

    GetUnderConn() interface{}
}

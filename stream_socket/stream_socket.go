/*
*  tcp或unix域套接字会话
*/

package stream_socket

import (
	   "net"
	   "reflect"
	   "time"
	   "sync"
	   "bufio"
	   "io"
	   "github.com/sniperHW/kendynet/util" 
	   "github.com/sniperHW/kendynet"
	   "sync/atomic"
	   //"fmt"
)

type StreamSocket struct {
	conn 			 net.Conn
	ud   			 interface{}
	sendQue         *util.BlockQueue
	receiver         kendynet.Receiver
	encoder          kendynet.EnCoder
	c                int32
	closed           bool
	started          bool
	recvTimeout      time.Duration
	sendTimeout      time.Duration
	mutex            sync.Mutex
	onClose          func (kendynet.StreamSession,string)
	onEvent          func (*kendynet.Event)
	closeReason      string
	closeChan        chan int          
}


func (this *StreamSocket) SetUserData(ud interface{}) {
	this.ud = ud
}

func (this *StreamSocket) GetUserData() interface{} {
	return this.ud
}

func (this *StreamSocket) LocalAddr() net.Addr {
	return this.conn.LocalAddr()
}

func (this *StreamSocket) RemoteAddr() net.Addr {
	return this.conn.RemoteAddr()
}

func (this *StreamSocket) Close(reason string, timeout time.Duration) error {
	this.mutex.Lock()
	if this.closed {
		this.mutex.Unlock()
		return kendynet.ErrSocketClose
	}

	if timeout < 0 {
		timeout = 0
	}

	this.closeReason = reason
	this.closed = true
	if this.sendQue.Len() > 0 {
		timeout = timeout * time.Second
		if timeout == 0 {
			this.sendQue.Clear()
		}
	} 
	this.sendQue.Close()
	this.mutex.Unlock()		
	if timeout > 0 {
		ticker := time.NewTicker(timeout)
		defer func () {
			ticker.Stop()
			if nil != this.onClose {
				this.onClose(this,this.closeReason)
			}
		}()
		for {
			select {
				case <- this.closeChan:
					return nil
				case <- ticker.C:
					this.conn.Close()
					return nil
			}
		}		
	} else {
		this.conn.Close()
		if nil != this.onClose {
			this.onClose(this,this.closeReason)
		}
		return nil		
	}
}

func (this *StreamSocket) SetReceiveTimeout(timeout time.Duration) {
	this.mutex.Lock()
	this.recvTimeout = timeout * time.Second
	this.mutex.Unlock()
}

func (this *StreamSocket) SetSendTimeout(timeout time.Duration) {
	this.mutex.Lock()	
	this.sendTimeout = timeout * time.Second
	this.mutex.Unlock()	
}
    
func (this *StreamSocket) SetCloseCallBack(cb func (kendynet.StreamSession, string)) {
	this.onClose = cb
}

func (this *StreamSocket) getReceiveTimeout() (timeout time.Duration){
	this.mutex.Lock()
	timeout = this.recvTimeout
	this.mutex.Unlock()
	return
}

func (this *StreamSocket) getSendTimeout() (timeout time.Duration) {
	this.mutex.Lock()	
	timeout = this.sendTimeout;
	this.mutex.Unlock()	
	return
}

func (this *StreamSocket) SetEventCallBack(cb func (*kendynet.Event)) {
	this.onEvent = cb
}

func (this *StreamSocket) SetEncoder(encoder kendynet.EnCoder) {
	this.encoder = encoder
}

func (this *StreamSocket) SetReceiver(r kendynet.Receiver) {
	this.receiver = r
}

func (this *StreamSocket) Send(o interface{}) error {
	if o == nil {
		return kendynet.ErrInvaildObject
	}

	if this.encoder == nil {
		return kendynet.ErrInvaildEncoder
	}

	msg,err := this.encoder.EnCode(o)

	if err != nil {
		return err
	}

	return this.SendMessage(msg)

}
	
func (this *StreamSocket) SendMessage(msg kendynet.Message) error {
	this.mutex.Lock()	
	defer func() {
		this.mutex.Unlock()
	}()
	if msg == nil {
		return kendynet.ErrInvaildBuff
	} else if this.closed {
		return kendynet.ErrSocketClose
	} else {
		if nil != this.sendQue.Add(msg) {
			return kendynet.ErrSocketClose
		}
	}
	return nil
}

func recvThreadFunc(session *StreamSocket) {

	for !session.sendQue.Closed() {

		recvTimeout := session.getReceiveTimeout()

		if recvTimeout > 0 {
			session.conn.SetReadDeadline(time.Now().Add(recvTimeout))
		}
		
		p,err := session.receiver.ReceiveAndUnpack(session)
		if session.sendQue.Closed() {
			//上层已经调用关闭，所有事件都不再传递上去
			break
		}
		if err != nil || p != nil {
			var event kendynet.Event
			event.Session = session
			if err != nil {
				event.EventType = kendynet.EventTypeError
				event.Data = err
			} else {
				event.EventType = kendynet.EventTypeMessage
				event.Data = p
			}
			/*出现错误不主动退出循环，除非用户调用了session.Close()		
	        * 避免用户遗漏调用Close(不调用Close会持续通告错误)
	        */	
			session.onEvent(&event)
		}
	}

	session.conn.Close()
	if atomic.AddInt32(&session.c,1) == 2 {
		session.closeChan <- 1
	}
}

func sendThreadFunc(session *StreamSocket) {

	writer := bufio.NewWriter(session.conn)
	localList := util.NewList()

	for {
		closed := session.sendQue.Get(localList)
		if closed && localList.Empty() {
			break
		}

		for !localList.Empty() {
			msg := localList.Pop().(kendynet.Message)
			data := msg.Bytes()
			for data != nil || (localList.Empty() && writer.Buffered() > 0) {
				if data != nil {
					var s int
					if len(data) > writer.Available() {
						s = writer.Available()
					} else {
						s = len(data)
					}
					writer.Write(data[:s])
					
					if s != len(data) {
						data = data[s:]
					} else {
						data = nil
					}
				}

				if writer.Available() == 0 || localList.Empty() {
					timeout := session.getSendTimeout()
					if timeout > 0 {
						session.conn.SetWriteDeadline(time.Now().Add(timeout))
					}
					err := writer.Flush()
					if err != nil && err != io.ErrShortWrite && !session.sendQue.Closed() {
						if err.(net.Error).Timeout() {
							err = kendynet.ErrSendTimeout
						}
						event := &kendynet.Event{Session:session,EventType:kendynet.EventTypeError,Data:err}
						session.onEvent(event)						
					}
				}
			}
		}
	}
	session.conn.Close()
	if atomic.AddInt32(&session.c,1) == 2 {	
		session.closeChan <- 1	
	}	
}


func (this *StreamSocket) Start() error {

	defer func(){
		this.mutex.Unlock()
	}()

	this.mutex.Lock()

	if this.closed {
		return kendynet.ErrSocketClose
	}

	if this.started {
		return kendynet.ErrStarted
	}

	if this.onEvent == nil {
		return kendynet.ErrNoOnEvent
	}

	if this.receiver == nil {
		return kendynet.ErrNoReceiver
	}

	this.started = true
	go sendThreadFunc(this)
	go recvThreadFunc(this)
	return nil
}

func NewStreamSocket(conn net.Conn)(kendynet.StreamSession){

	switch conn.(type) {
		case *net.TCPConn:
			break
		case *net.UnixConn:
			break
		default:
			kendynet.Logger.Errorf(util.FormatFileLine("unsupport conn type:%s\n",reflect.TypeOf(conn).String()))
			return nil
	}

	session 			:= new(StreamSocket)
	session.conn 		 = conn
	session.sendQue      = util.NewBlockQueue()
	session.closeChan    = make(chan int,1)
	return session
}

func (this *StreamSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *StreamSocket) Read(b []byte) (int, error) {
	return this.conn.Read(b)
}




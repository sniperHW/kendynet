/*
*  tcp或unix域套接字会话
*/

package kendynet

import (
	   "net"
	  // "fmt"
	   "time"
	   "sync"
	   "bufio"
	   "io"
	   "github.com/sniperHW/kendynet/util" 
)

type StreamSocket struct {
	conn 			 net.Conn
	ud   			 interface{}
	sendQue         *util.BlockQueue
	receiver         Receiver
	encoder          EnCoder
	sendStop         bool
	recvStop         bool
	closed           bool
	started          bool
	closeDeadline    time.Time
	recvTimeout      time.Duration
	sendTimeout      time.Duration
	mutex            sync.Mutex
	onClose          func (StreamSession,string)
	onEvent          func (*Event)
	closeReason      string           
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
	defer func(){
		this.mutex.Unlock()
	}()

	this.mutex.Lock()

	if this.closed {
		return ErrSocketClose
	}

	this.closeReason = reason
	this.closed = true

	if this.started {
		if timeout == 0 {
			switch this.conn.(type) {
			case *net.TCPConn:
				this.conn.(*net.TCPConn).CloseWrite()
				break
			case *net.UnixConn:
				this.conn.(*net.UnixConn).CloseWrite()
				break
			default:
				this.conn.Close()
			}			
		} else {
			this.closeDeadline = time.Now().Add(timeout)
		}
	} else {
		this.conn.Close()
		if nil != this.onClose {
			this.onClose(this,this.closeReason)
		}
	}
	
	this.sendQue.Close()

	return nil
}

func (this *StreamSocket) SetReceiveTimeout(timeout time.Duration) {
	this.recvTimeout = timeout * time.Second
}

func (this *StreamSocket) SetSendTimeout(timeout time.Duration) {
	this.sendTimeout = timeout * time.Second
}
    
func (this *StreamSocket) SetCloseCallBack(cb func (StreamSession, string)) {
	this.onClose = cb
}

func (this *StreamSocket) SetEventCallBack(cb func (*Event)) {
	this.onEvent = cb
}

func (this *StreamSocket) SetEncoder(encoder EnCoder) {
	this.encoder = encoder
}

func (this *StreamSocket) SetReceiver(r Receiver) {
	this.receiver = r
}

func (this *StreamSocket) Send(o interface{}) error {
	if o == nil {
		return ErrInvaildObject
	}

	if this.encoder == nil {
		return ErrInvaildEncoder
	}

	msg,err := this.encoder.EnCode(o)

	if err != nil {
		return err
	}

	return this.SendMessage(msg)

}
	
func (this *StreamSocket) SendMessage(msg Message) error {
	if msg == nil {
		return ErrInvaildBuff
	} else if this.sendStop || this.closed {
		return ErrSocketClose
	} else {
		if nil != this.sendQue.Add(msg) {
			return ErrSocketClose
		}
	}
	return nil
}

func recvThreadFunc(session *StreamSocket) {

	defer func() {
		session.conn.Close()
		session.mutex.Lock()
		session.recvStop = true
		if session.sendStop && nil != session.onClose {
			session.onClose(session,session.closeReason)
		} 
		session.mutex.Unlock()
	}()

	for !session.closed {
		if session.recvTimeout > 0 {
			session.conn.SetReadDeadline(time.Now().Add(session.recvTimeout))
		}
		
		p,err := session.receiver.ReceiveAndUnpack(session)
		if session.closed {
			break
		}

		if err != nil || p != nil {
			var event Event
			event.Session = session
			if err != nil {
				event.EventType = EventTypeError
				event.Data = err
			} else {
				event.EventType = EventTypeMessage
				event.Data = p
			}
			/*出现错误不主动退出循环，除非用户调用了session.Close()		
	        * 避免用户遗漏调用Close(不调用Close会持续通告错误)
	        */	
			session.onEvent(&event)
		}
	}
}


func doSend(session *StreamSocket,writer *bufio.Writer,timeout time.Time) error {

	/*
	 *  for循环的必要性
	 *  假设需要发送唯一一个非常大的包，而接收方非常慢，Flush将会返回io.ErrShortWrite
	 *  如果此时退出doSend，sendThreadFunc将会永远阻塞在session.sendQue.Get(&writeList)上
	 *  因为无法继续执行doSend，对端将永远无法接收到完整的包
	*/
	for {

		session.conn.SetWriteDeadline(timeout)

		err := writer.Flush()

		if err != nil {

			if err.(net.Error).Timeout() {
				return ErrSendTimeout
			}
	
			if err != io.ErrShortWrite {
				return err
			}
			//如果ShortWrite继续尝试发送
		}else {
			return nil
		}
	}

	return nil
}

func writeToWriter(writer *bufio.Writer,buffer []byte) error {

	sizeToWrite := len(buffer)

	idx := 0

	for {

		n, err := writer.Write(buffer[idx:sizeToWrite])

		if err != nil {
			return err
		}

		if n >= sizeToWrite {
			break
		}

		sizeToWrite -= n
		idx += n
	}

	return nil

}

func sendThreadFunc(session *StreamSocket) {
	defer func() {
		session.conn.Close()
		session.mutex.Lock()
		session.sendStop = true
		if session.recvStop && nil != session.onClose {
			session.onClose(session,session.closeReason)
		} 
		session.mutex.Unlock()
	}()

	writer := bufio.NewWriter(session.conn)

	var timeout time.Time

	localList := util.NewList()

	for {

		closed := session.sendQue.Get(localList)

		if closed && session.closeDeadline.IsZero() {
			//关闭，丢弃所有待发送数据
			return
		}
		
		for !localList.Empty() {
			msg := localList.Pop().(Message)
			if msg.Bytes() != nil {
				if err := writeToWriter(writer,msg.Bytes()); err != nil {
					event := &Event{Session:session,EventType:EventTypeError,Data:err}
					session.onEvent(event)
					return
				}
			}
		}

		if closed && 0 == writer.Buffered() {
			//没有数据需要发送了,退出
			return
		}

		if closed {
			timeout = session.closeDeadline
		} else {
			timeout = time.Now().Add(session.sendTimeout)				
		}

		if err := doSend(session,writer,timeout); err != nil {
			if closed {
				return
			} else {
				event := &Event{Session:session,EventType:EventTypeError,Data:err}
				session.onEvent(event)
			}
		}
	}

}


func (this *StreamSocket) Start() error {

	defer func(){
		this.mutex.Unlock()
	}()

	this.mutex.Lock()

	if this.closed {
		return ErrSocketClose
	}

	if this.started {
		return ErrStarted
	}

	if this.onEvent == nil {
		return ErrNoOnPacket
	}

	if this.receiver == nil {
		return ErrNoReceiver
	}

	this.started = true
	go sendThreadFunc(this)
	go recvThreadFunc(this)
	return nil
}

func NewStreamSocket(conn net.Conn)(StreamSession){
	session 			:= new(StreamSocket)
	session.conn 		 = conn
	session.sendQue      = util.NewBlockQueue()
	session.sendTimeout  = DefaultSendTimeout * time.Second
	return session
}

func (this *StreamSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *StreamSocket) Read(b []byte) (int, error) {
	return this.conn.Read(b)
}




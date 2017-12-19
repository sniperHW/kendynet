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
	   //"fmt"
)

const (
	started      = (1 << 0)
	closed       = (1 << 1)
)

type StreamSocket struct {
	conn 			  net.Conn
	ud   			  interface{}
	sendQue          *kendynet.SendQueue
	receiver          kendynet.Receiver
	encoder           kendynet.EnCoder
	sendBuffProcessor SendBuffProcessor
	flag              int32
	option            kendynet.SessionOption
	mutex             sync.Mutex
	onClose           func (kendynet.StreamSession,string)
	onEvent           func (*kendynet.Event)
	closeReason       string
	sendCloseChan     chan int         
}


func (this *StreamSocket) SetUserData(ud interface{}) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.ud = ud
}

func (this *StreamSocket) GetUserData() (ud interface{}) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	ud = this.ud
	return this.ud
}

func (this *StreamSocket) LocalAddr() net.Addr {
	return this.conn.LocalAddr()
}

func (this *StreamSocket) RemoteAddr() net.Addr {
	return this.conn.RemoteAddr()
}

func (this *StreamSocket) isClosed() (ret bool) {
	this.mutex.Lock()
	ret = (this.flag & closed) > 0
	this.mutex.Unlock()
	return
}

func (this *StreamSocket) doClose() {
	this.conn.Close()
	this.mutex.Lock()
	onClose := this.onClose
	this.mutex.Unlock()
	if nil != onClose {
		onClose(this,this.closeReason)
	}
} 

func (this *StreamSocket) shutdownRead() {
	switch this.conn.(type) {
	case *net.TCPConn:
		this.conn.(*net.TCPConn).CloseRead()
		break
	case *net.UnixConn:
		this.conn.(*net.UnixConn).CloseRead()
		break
	}
} 

func (this *StreamSocket) Close(reason string, timeout time.Duration) {
	this.mutex.Lock()
	if (this.flag & closed) > 0 {
		this.mutex.Unlock()
		return
	}

	this.closeReason = reason
	this.flag |= closed
	this.sendQue.Close()
	this.mutex.Unlock()
	if this.sendQue.Len() > 0 {
		timeout = timeout * time.Second
		if timeout <= 0 {
			this.sendQue.Clear()
		}
	} 		
	if timeout > 0 {
		this.shutdownRead()
		ticker := time.NewTicker(timeout)
		go func() {
			/*
			 *	timeout > 0,sendThread最多需要经过timeout秒之后才会结束， 
			 *	为了避免阻塞调用Close的goroutine,启动一个新的goroutine在chan上等待事件
			*/
			select {
				case <- this.sendCloseChan:
				case <- ticker.C:
			}
			ticker.Stop()
			this.doClose()		
		}()
	} else{
		this.doClose()
	}
	
}
    
func (this *StreamSocket) SetCloseCallBack(cb func (kendynet.StreamSession, string)) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.onClose = cb
}

func (this *StreamSocket) SetEncoder(encoder kendynet.EnCoder) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.encoder = encoder
}

func (this *StreamSocket) SetReceiver(r kendynet.Receiver) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	if (this.flag & started) > 0 {
		return
	}	
	this.receiver = r
}

func (this *StreamSocket) SetSendBuffProcessor(processor SendBuffProcessor) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.sendBuffProcessor = processor
}


func (this *StreamSocket) sendMessage(msg kendynet.Message) error {
	if msg == nil {
		return kendynet.ErrInvaildBuff
	} else if (this.flag & closed) > 0 {
		return kendynet.ErrSocketClose
	} else if nil != this.sendQue.Add(msg) {
		return kendynet.ErrSocketClose
	}
	return nil
}

func (this *StreamSocket) Send(o interface{}) error {
	if o == nil {
		return kendynet.ErrInvaildObject
	}

	this.mutex.Lock()	
	defer this.mutex.Unlock()	

	if this.encoder == nil {
		return kendynet.ErrInvaildEncoder
	}

	msg,err := this.encoder.EnCode(o)

	if err != nil {
		return err
	}

	return this.sendMessage(msg)
}
	
func (this *StreamSocket) SendMessage(msg kendynet.Message) error {
	this.mutex.Lock()	
	defer this.mutex.Unlock()
	return this.sendMessage(msg)
}

func recvThreadFunc(session *StreamSocket) {

	for !session.isClosed() {

		recvTimeout := session.option.RecvTimeout

		if recvTimeout > 0 {
			session.conn.SetReadDeadline(time.Now().Add(recvTimeout))
		}
		
		p,err := session.receiver.ReceiveAndUnpack(session)
		if session.isClosed() {
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
}

func sendThreadFunc(session *StreamSocket) {

	defer func(){
		session.sendCloseChan <- 1
	}()

	writer := bufio.NewWriter(session.conn)
	for {
		closed,localList := session.sendQue.Get()
		size := len(localList)
		if closed && size == 0 {
			break
		}

		if nil != session.sendBuffProcessor {
			localList = session.sendBuffProcessor.Process(localList)
			size = len(localList)
		}

		for i := 0; i < size; i++ {
			msg := localList[i]//.(kendynet.Message)
			data := msg.Bytes()
			for data != nil || (i == (size - 1) && writer.Buffered() > 0) {
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

				if writer.Available() == 0 || i == (size - 1) {
					timeout := session.option.SendTimeout
					if timeout > 0 {
						session.conn.SetWriteDeadline(time.Now().Add(timeout))
					}
					err := writer.Flush()
					if err != nil && err != io.ErrShortWrite {
						if session.sendQue.Closed() {
							return
						}
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
}


func (this *StreamSocket) Start(eventCB func (*kendynet.Event)) error {

	this.mutex.Lock()
	defer this.mutex.Unlock()

	if (this.flag & closed) > 0 {
		return kendynet.ErrSocketClose
	}

	if (this.flag & started) > 0 {
		return kendynet.ErrStarted
	}

	if eventCB == nil {
		return kendynet.ErrNoOnEvent
	}

	if this.receiver == nil {
		return kendynet.ErrNoReceiver
	}

	this.onEvent = eventCB
	this.flag |= started
	go sendThreadFunc(this)
	go recvThreadFunc(this)
	return nil
}

func NewStreamSocket(conn net.Conn,option ...kendynet.SessionOption)(kendynet.StreamSession){

	switch conn.(type) {
		case *net.TCPConn:
			break
		case *net.UnixConn:
			break
		default:
			kendynet.Logger.Errorf(util.FormatFileLine("unsupport conn type:%s\n",reflect.TypeOf(conn).String()))
			return nil
	}

	session 			 := new(StreamSocket)
	session.conn 		  = conn
	session.sendQue       = kendynet.NewSendQueue()
	session.sendCloseChan = make(chan int,1)

	if len(option) > 0 {
		session.option = option[0]
	}

	return session
}

func (this *StreamSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *StreamSocket) Read(b []byte) (int, error) {
	return this.conn.Read(b)
}




/*
*  websocket会话
*/

package websocket


import (
	   "fmt"
	   "net"
	   "time"
	   "sync"
	   "github.com/sniperHW/kendynet/util" 
	   "github.com/sniperHW/kendynet"
	   gorilla "github.com/gorilla/websocket"
	   "sync/atomic"
)

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	WSTextMessage = 1

	// BinaryMessage denotes a binary data message.
	WSBinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	WSCloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	WSPingMessage = 9

	// PongMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	WSPongMessage = 10
)

/*
*  WSMessage与普通的ByteBuffer Msg的区别在于多了一个messageType字段   
*/
type WSMessage struct {
	messageType int
	buff       *kendynet.ByteBuffer
}

func (this *WSMessage) Bytes() []byte {
	return this.buff.Bytes()
}

func (this *WSMessage) PutBytes(idx uint64,value []byte)(error){
	return this.buff.PutBytes(idx,value)
}

func (this *WSMessage) GetBytes(idx uint64,size uint64) ([]byte,error) {
	return this.buff.GetBytes(idx,size)
}

func (this *WSMessage) PutString(idx uint64,value string)(error){
	return this.buff.PutString(idx,value)
}

func (this *WSMessage) GetString(idx uint64,size uint64) (string,error) {
	return this.buff.GetString(idx,size)
}

func NewMessage(messageType int,optional ...interface{}) *WSMessage {
	buff := kendynet.NewByteBuffer(optional...)
	if nil == buff {
		fmt.Printf("nil == buff\n")
		return nil
	}
	return &WSMessage{messageType:messageType,buff:buff}
}

type WebSocket struct {
	conn *gorilla.Conn
	ud   interface{}
	sendQue          *util.BlockQueue
	receiver          kendynet.Receiver
	encoder           kendynet.EnCoder
	sendStop          bool
	recvStop          bool
	closed            bool
	started           bool
	closeDeadline     time.Time 
	recvTimeout       time.Duration
	sendTimeout       time.Duration
	mutex             sync.Mutex
	onClose           func (kendynet.StreamSession,string)
	onEvent           func (*kendynet.Event)
	closeReason       string
	c                 int32
	closeChan         chan int	
}

func (this *WebSocket) SetUserData(ud interface{}) {
	this.ud = ud
}

func (this *WebSocket) GetUserData() interface{} {
	return this.ud
}

func (this *WebSocket) LocalAddr() net.Addr {
	return this.conn.LocalAddr()
}

func (this *WebSocket) RemoteAddr() net.Addr {
	return this.conn.RemoteAddr()
}

func (this *WebSocket) SetReceiveTimeout(timeout time.Duration) {
	this.mutex.Lock()	
	this.recvTimeout = timeout * time.Second
	this.mutex.Unlock()
}

func (this *WebSocket) SetSendTimeout(timeout time.Duration) {
	this.mutex.Lock()	
	this.sendTimeout = timeout * time.Second
	this.mutex.Unlock()
}

func (this *WebSocket) getReceiveTimeout() (timeout time.Duration){
	this.mutex.Lock()
	timeout = this.recvTimeout
	this.mutex.Unlock()
	return
}

func (this *WebSocket) getSendTimeout() (timeout time.Duration) {
	this.mutex.Lock()	
	timeout = this.sendTimeout;
	this.mutex.Unlock()
	return	
}

func (this *WebSocket) getCloseDeadline() (closeDeadline time.Time) {
	this.mutex.Lock()	
	closeDeadline = this.closeDeadline;
	this.mutex.Unlock()
	return	
}
    
func (this *WebSocket) SetCloseCallBack(cb func (kendynet.StreamSession, string)) {
	this.onClose = cb
}

func (this *WebSocket) SetEventCallBack(cb func (*kendynet.Event)) {
	this.onEvent = cb
}

func (this *WebSocket) SetEncoder(encoder kendynet.EnCoder) {
	this.encoder = encoder
}

func (this *WebSocket) SetReceiver(r kendynet.Receiver) {
	this.receiver = r
}

func (this *WebSocket) Send(o interface{}) error {
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
	
func (this *WebSocket) SendMessage(msg kendynet.Message) error {
	this.mutex.Lock()	
	defer func() {
		this.mutex.Unlock()
	}()	
	if msg == nil {
		return kendynet.ErrInvaildBuff
	} else if this.closed {
		return kendynet.ErrSocketClose
	} else {
		switch msg.(type) {
			case *WSMessage:
				if nil == msg.(*WSMessage) {
					return ErrInvaildWSMessage
				}
				if nil != this.sendQue.Add(msg) {
					return kendynet.ErrSocketClose
				}
				break
			default:
				return ErrInvaildWSMessage
		}
	}
	return nil
}

func recvThreadFunc(session *WebSocket) {
	atomic.AddInt32(&session.c,1)
	for !session.sendQue.Closed() {
		recvTimeout := session.getReceiveTimeout()
		if recvTimeout > 0 {
			session.conn.SetReadDeadline(time.Now().Add(recvTimeout))
		}
		
		p,err := session.receiver.ReceiveAndUnpack(session)
		if session.sendQue.Closed() {
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
	if atomic.AddInt32(&session.c,-1) == 0 {
		session.closeChan <- 1
	}	
}

func sendThreadFunc(session *WebSocket) {
	atomic.AddInt32(&session.c,1)
	localList := util.NewList()	
	for {

		closed := session.sendQue.Get(localList)
		if closed && localList.Empty() {
			break
		}

		for !localList.Empty() {
			var err error
			msg := localList.Pop().(*WSMessage)
			timeout := session.getSendTimeout()
			if msg.messageType == WSBinaryMessage || msg.messageType == WSTextMessage {
				if timeout > 0 {
					session.conn.SetWriteDeadline(time.Now().Add(timeout))
				}
				err = session.conn.WriteMessage(msg.messageType,msg.Bytes())
			} else if msg.messageType == WSCloseMessage || msg.messageType == WSPingMessage || msg.messageType == WSPingMessage {
				var deadline time.Time
				if timeout > 0 {
					deadline = time.Now().Add(timeout)
				}
				err = session.conn.WriteControl(msg.messageType,msg.Bytes(),deadline)
			}

			if err != nil && msg.messageType != WSCloseMessage && !session.sendQue.Closed() {
				event := &kendynet.Event{Session:session,EventType:kendynet.EventTypeError,Data:err}
				session.onEvent(event)
			}			
		}
	}
	session.conn.Close()	
	if atomic.AddInt32(&session.c,-1) == 0 {
		session.closeChan <- 1
	}	
}

func (this *WebSocket) Start() error {

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

func (this *WebSocket) Close(reason string, timeout time.Duration) error {
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
	timeout = timeout * time.Second
	if this.sendQue.Len() > 0 {
		timeout = timeout * time.Second
		if timeout == 0 {
			this.sendQue.Clear()
		}
	}
	if timeout > 0 {
		message := gorilla.FormatCloseMessage(1000, reason)
		this.sendQue.Add(NewMessage(WSCloseMessage,message))
		this.sendQue.Close()	
		if !this.started {
			go sendThreadFunc(this)
		}
		this.mutex.Unlock()				
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
		this.mutex.Unlock()		
		this.sendQue.Close()
		this.conn.Close()
		if nil != this.onClose {
			this.onClose(this,this.closeReason)
		}
		return nil	
	}
}


func NewWSSocket(conn *gorilla.Conn)(kendynet.StreamSession){
	session 			:= new(WebSocket)
	session.conn 		 = conn
	session.sendQue      = util.NewBlockQueue()
	session.closeChan    = make(chan int,1)
	session.conn.SetCloseHandler(func(code int, text string) error {
		return fmt.Errorf("peer close reason[%s]",text)
	})
	return session
}

func (this *WebSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *WebSocket) ReadMessage() (messageType int, p []byte, err error) {
	return this.conn.ReadMessage()
}


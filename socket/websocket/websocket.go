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
)

const (
	started      = (1 << 0)
	closed       = (1 << 1)
	wclosed      = (1 << 2)
	rclosed      = (1 << 3)
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

func (this *WSMessage) Type() int {
	return this.messageType
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
	flag              int32
//	option            kendynet.SessionOption
    SendTimeout 	  time.Duration
    RecvTimeout       time.Duration	
	mutex             sync.Mutex
	onClose           func (kendynet.StreamSession,string)
	onEvent           func (*kendynet.Event)
	closeReason       string
	sendCloseChan     chan int 
	postQueue         [] kendynet.Message
}

func (this *WebSocket) SetUserData(ud interface{}) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.ud = ud
}

func (this *WebSocket) GetUserData() (ud interface{}) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	ud = this.ud
	return this.ud
}

func (this *WebSocket) LocalAddr() net.Addr {
	return this.conn.LocalAddr()
}

func (this *WebSocket) RemoteAddr() net.Addr {
	return this.conn.RemoteAddr()
}


func (this *WebSocket) SetCloseCallBack(cb func (kendynet.StreamSession, string)) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.onClose = cb
}

func (this *WebSocket) SetEncoder(encoder kendynet.EnCoder) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.encoder = encoder
}

func (this *WebSocket) SetReceiver(r kendynet.Receiver) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	if (this.flag & started) > 0 {
		return
	}	
	this.receiver = r
}

func (this *WebSocket) flush() {
	size := len(this.postQueue)
	if size > 0 {
		for i := 0; i < size; i++ {
			this.sendQue.Add(this.postQueue[i])
		}
		this.postQueue = this.postQueue[0:0]
	}
}

func (this *WebSocket) Flush() {
	this.mutex.Lock()	
	defer this.mutex.Unlock()
	this.flush()
}

func (this *WebSocket) postSendMessage(msg kendynet.Message) error {
	if msg == nil {
		return kendynet.ErrInvaildBuff
	} else if (this.flag & closed) > 0 || (this.flag & wclosed) > 0 {
		return kendynet.ErrSocketClose
	} else {
		this.postQueue = append(this.postQueue,msg)
	}
	return nil
}

func (this *WebSocket) PostSend(o interface{}) error {
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

	return this.postSendMessage(msg)
}
	
func (this *WebSocket) PostSendMessage(msg kendynet.Message) error {
	this.mutex.Lock()	
	defer this.mutex.Unlock()
	return this.postSendMessage(msg)
}
    
func (this *WebSocket) sendMessage(msg kendynet.Message) error {
	if msg == nil {
		return kendynet.ErrInvaildBuff
	} else if (this.flag & closed) > 0 || (this.flag & wclosed) > 0 {
		return kendynet.ErrSocketClose
	} else {
		this.flush()
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

func (this *WebSocket) Send(o interface{}) error {
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

	
func (this *WebSocket) SendMessage(msg kendynet.Message) error {
	this.mutex.Lock()	
	defer this.mutex.Unlock()
	return this.sendMessage(msg)
}

func recvThreadFunc(session *WebSocket) {
	for !session.isClose() {
		recvTimeout := session.RecvTimeout
		if recvTimeout > 0 {
			session.conn.SetReadDeadline(time.Now().Add(recvTimeout))
		}
		
		p,err := session.receiver.ReceiveAndUnpack(session)
		if session.isClose() {
			break
		}

		if err != nil || p != nil {
			var event kendynet.Event
			event.Session = session
			if err != nil {
				event.EventType = kendynet.EventTypeError
				event.Data = err
				if !kendynet.IsNetTimeout(err) {
					kendynet.Errorf("ReceiveAndUnpack error:%s\n",err.Error())
					session.mutex.Lock()
					session.flag |= (rclosed | wclosed)
					session.mutex.Unlock()
				}
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

func sendThreadFunc(session *WebSocket) {	
	for {
		closed,localList := session.sendQue.Get()
		size := len(localList)
		if closed && size == 0 {
			break
		}

		for i := 0; i < size; i++ {
			var err error			
			msg := localList[i].(*WSMessage)
			timeout := session.SendTimeout
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

			if err != nil && msg.messageType != WSCloseMessage {
				if session.sendQue.Closed() {
					return
				}

				if kendynet.IsNetTimeout(err) {
					err = kendynet.ErrSendTimeout
				} else {
					kendynet.Errorf("websocket write error:%s\n",err.Error())
					session.mutex.Lock()
					session.flag |= wclosed
					session.mutex.Unlock()							
				}

				event := &kendynet.Event{Session:session,EventType:kendynet.EventTypeError,Data:err}
				session.onEvent(event)
			}		
		}
	}
	session.sendCloseChan <- 1
}

func (this *WebSocket) Start(eventCB func (*kendynet.Event)) error {

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
	this.flag |= started
	this.onEvent = eventCB
	go sendThreadFunc(this)
	go recvThreadFunc(this)
	
	return nil
}

func (this *WebSocket) isClose() (ret bool) {
	this.mutex.Lock()
	ret = (this.flag & closed) > 0
	this.mutex.Unlock()
	return
}

func (this *WebSocket) doClose() {
	this.conn.Close()
	this.mutex.Lock()
	onClose := this.onClose
	this.mutex.Unlock()
	if nil != onClose {
		onClose(this,this.closeReason)
	}
} 

func (this *WebSocket) shutdownRead() {
	underConn := this.conn.UnderlyingConn()
	switch underConn.(type) {
		case *net.TCPConn:
			underConn.(*net.TCPConn).CloseRead()
			break
		case *net.UnixConn:
			underConn.(*net.UnixConn).CloseRead()
			break
	}
}

func (this *WebSocket) Close(reason string, delay time.Duration) {
	this.mutex.Lock()
	if (this.flag & closed) > 0 {
		this.mutex.Unlock()
		return
	}

	this.closeReason = reason
	this.flag |= (closed | rclosed)
	if (this.flag & wclosed) > 0 {
		delay = 0 //写端已经关闭忽略delay参数
	} else {
		delay = delay * time.Second
	}

	if delay > 0 {
		this.shutdownRead()
		message := gorilla.FormatCloseMessage(1000, reason)
		this.flush()
		this.sendQue.Add(NewMessage(WSCloseMessage,message))
		this.sendQue.Close()
		ticker := time.NewTicker(delay)
		if (this.flag & started) == 0 {
			go sendThreadFunc(this)
		}
		this.mutex.Unlock()
		go func() {
			select {
				case <- this.sendCloseChan:
				case <- ticker.C:
			}
			ticker.Stop()
			this.doClose()
			return		
		}()		
	} else {
		this.sendQue.Close()
		this.mutex.Unlock()
		this.doClose()
		return				
	}
}


func NewWSSocket(conn *gorilla.Conn)(kendynet.StreamSession){
	if nil == conn {
		return nil
	} else { 
		conn.SetCloseHandler(func(code int, text string) error {
			return fmt.Errorf("peer close reason[%s]",text)
		})
		return &WebSocket{
			conn : conn,
			sendQue : util.NewBlockQueue(),
			sendCloseChan : make(chan int,1),
		}
	}
}

func (this *WebSocket) SetPingHandler(h func(appData string) error) {
	this.conn.SetPingHandler(h)
}

func (this *WebSocket) SetPongHandler(h func(appData string) error) {
	this.conn.SetPongHandler(h)
}

func (this *WebSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *WebSocket) Read() (messageType int, p []byte, err error) {
	return this.conn.ReadMessage()
}

func (this *WebSocket) SetRecvTimeout(timeout time.Duration) {
	this.RecvTimeout = timeout * time.Millisecond
}

func (this *WebSocket) SetSendTimeout(timeout time.Duration) {
	this.SendTimeout = timeout * time.Millisecond
}


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
	name              string
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
	this.recvTimeout = timeout * time.Second
}

func (this *WebSocket) SetSendTimeout(timeout time.Duration) {
	this.sendTimeout = timeout * time.Second
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
	if msg == nil {
		return kendynet.ErrInvaildBuff
	} else if this.sendStop || this.closed {
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

func wsRecvThreadFunc(session *WebSocket) {

	defer func() {
		session.mutex.Lock()
		session.recvStop = true
		if session.sendStop && nil != session.onClose {
			session.onClose(session,session.closeReason)
		} 
		session.mutex.Unlock()
	}()

	for !session.sendQue.Closed() {
		if session.recvTimeout > 0 {
			session.conn.SetReadDeadline(time.Now().Add(session.recvTimeout))
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
}

func wsSendThreadFunc(session *WebSocket) {
	defer func() {
		session.mutex.Lock()
		session.conn.Close()
		session.sendStop = true
		if session.recvStop && nil != session.onClose {
			session.onClose(session,session.closeReason)
		}
		session.mutex.Unlock()
	}()


	var timeout time.Time

	localList := util.NewList()
	
	for {

		closed := session.sendQue.Get(localList)

		//fmt.Printf("localList size:%d\n",localList.Len())

		if closed {
			if session.closeDeadline.IsZero() {
				//关闭，丢弃所有待发送数据
				return
			} else {
				timeout = session.closeDeadline
			}
		} else if session.sendTimeout > 0 {
			timeout = time.Now().Add(session.sendTimeout)				
		}

		for !localList.Empty() {
			var err error
			msg := localList.Pop().(*WSMessage)
			if msg.messageType == WSBinaryMessage || msg.messageType == WSTextMessage {
				session.conn.SetWriteDeadline(timeout)
				err = session.conn.WriteMessage(msg.messageType,msg.Bytes())
			} else if msg.messageType == WSCloseMessage || msg.messageType == WSPingMessage || msg.messageType == WSPingMessage {
				err = session.conn.WriteControl(msg.messageType,msg.Bytes(),timeout)
				if msg.messageType == WSCloseMessage {
					return
				}
			}
			if err != nil {
				if closed {
					return 
				} else {
					event := &kendynet.Event{Session:session,EventType:kendynet.EventTypeError,Data:err}
					session.onEvent(event)
					if session.sendQue.Closed() {
						break
					}
				}
			}
		}
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
	go wsSendThreadFunc(this)
	go wsRecvThreadFunc(this)
	return nil
}

func (this *WebSocket) Close(reason string, timeout time.Duration) error {
	defer func(){
		this.mutex.Unlock()
	}()

	this.mutex.Lock()

	if this.closed {
		return kendynet.ErrSocketClose
	}

	defer func(){
		this.sendQue.Close()
	}()

	this.closeReason = reason
	this.closed = true

	timeout = timeout * time.Second

	if this.started {
		if timeout == 0 {
			underConn := this.conn.UnderlyingConn()
			switch underConn.(type) {
				case *net.TCPConn:
					underConn.(*net.TCPConn).CloseWrite()
					break
				case *net.UnixConn:
					underConn.(*net.UnixConn).CloseWrite()
					break
				default:
					this.conn.Close()
			}			
		} else {
			this.closeDeadline = time.Now().Add(timeout)
			message := gorilla.FormatCloseMessage(1000, reason)
			err := this.sendQue.Add(NewMessage(WSCloseMessage,message))
			if err != nil {
				this.conn.Close()
			}			
		}
	} else {
		if timeout > 0 {
			//timeout > 0执行优雅关闭
			this.closeDeadline = time.Now().Add(timeout)
			message := gorilla.FormatCloseMessage(1000, reason)
			err := this.sendQue.Add(NewMessage(WSCloseMessage,message))
			if err == nil {
				go wsSendThreadFunc(this)
			} else {
				this.conn.Close()
				if nil != this.onClose {
					this.onClose(this,this.closeReason)
				}			
				return err
			}
		} else {
			//否则立即关闭
			this.conn.Close()
			if nil != this.onClose {
				this.onClose(this,this.closeReason)
			}
			return nil			
		}
	}	

	return nil
}


func NewWSSocket(conn *gorilla.Conn)(kendynet.StreamSession){
	session 			:= new(WebSocket)
	session.conn 		 = conn
	session.sendQue      = util.NewBlockQueue()
	session.sendTimeout  = kendynet.DefaultSendTimeout * time.Second
	session.conn.SetCloseHandler(func(code int, text string) error {
		return fmt.Errorf("peer close reason[%s]",text)
	})
	session.name 		 = session.LocalAddr().String() + "<->" + session.RemoteAddr().String()
	return session
}

func (this *WebSocket) GetUnderConn() interface{} {
	return this.conn
}

func (this *WebSocket) ReadMessage() (messageType int, p []byte, err error) {
	return this.conn.ReadMessage()
}

/*
*   实现RPCChannel接口,可用于RPC通信
*/

func (this *WebSocket) SendRPCRequest(message interface {}) error {
	return this.Send(message)
}

func (this *WebSocket) SendRPCResponse(message interface {}) error {
	return this.Send(message)
}

func (this *WebSocket) Name() string {
	return this.name
}


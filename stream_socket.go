/*
*  tcp或unix域套接字会话
*/

package kendynet

import (
	   "net"
	   "fmt"
	   "time"
	   "sync"
	   "bufio"
	   "io"
	   "github.com/sniperHW/kendynet/util" 
)

type streamSocket struct {
	conn 			 net.Conn
	ud   			 interface{}
	sendQue         *util.Queue
	receiver         Receiver
	sendStop         bool
	recvStop         bool
	closed           bool
	started          bool
	finalSendTimeout int64   //sec
	recvTimeout      int64
	sendTimeout      int64
	mutex            sync.Mutex
	onClose          func (StreamSession,string)
	onPacket         func (StreamSession,interface{},error)
	closeReason      string           
}


func (this *streamSocket) SetUserData(ud interface{}) {
	this.ud = ud
}

func (this *streamSocket) GetUserData() interface{} {
	return this.ud
}

func (this *streamSocket) LocalAddr() net.Addr {
	return this.conn.LocalAddr()
}

func (this *streamSocket) RemoteAddr() net.Addr {
	return this.conn.RemoteAddr()
}

func (this *streamSocket) Close(reason string, timeout int64) error {
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
		this.finalSendTimeout = timeout
		if this.finalSendTimeout == 0 {
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

func (this *streamSocket) SetReceiveTimeout(timeout int64) {
	this.recvTimeout = timeout
}

func (this *streamSocket) SetSendTimeout(timeout int64) {
	this.sendTimeout = timeout
}
    
func (this *streamSocket) SetCloseCallBack(cb func (StreamSession, string)) {
	this.onClose = cb
}

func (this *streamSocket) SetPacketCallBack(cb func (StreamSession, interface{},error)) {
	this.onPacket = cb
}

func (this *streamSocket) SetReceiver(r Receiver) {
	this.receiver = r
}

func (this *streamSocket) Send(o interface{},encoder EnCoder) error {
	if o == nil {
		return ErrInvaildObject
	}

	if encoder == nil {
		return ErrInvaildEncoder
	}

	buff,err := encoder.EnCode(o)

	if err != nil {
		return err
	}

	return this.SendBuff(buff)

}
	
func (this *streamSocket) SendBuff(b *ByteBuffer) error {
	if b == nil {
		return ErrInvaildBuff
	} else if this.sendStop || this.closed {
		return ErrSocketClose
	} else {
		if nil != this.sendQue.Add(b) {
			return ErrSocketClose
		}
	}
	return nil
}

func recvThreadFunc(session *streamSocket) {

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
			t := time.Now()
			deadline := t.Add(time.Second * time.Duration(session.recvTimeout))
			session.conn.SetReadDeadline(deadline)
		}
		
		p,err := session.receiver.ReceiveAndUnpack(session)
		if session.closed {
			break
		}

		if err != nil {
			session.onPacket(session,nil,err)
			/*出现错误不主动退出循环，除非用户调用了session.Close()		
	        * 避免用户遗漏调用Close(不调用Close会持续通告错误)
	        */	
		} else if p != nil {
			session.onPacket(session,p,err)
		}
	}
}


func doSend(session *streamSocket,writer *bufio.Writer,timeout int64) error {

	/*
	 *  for循环的必要性
	 *  假设需要发送唯一一个非常大的包，而接收方非常慢，Flush将会返回io.ErrShortWrite
	 *  如果此时退出doSend，sendThreadFunc将会永远阻塞在session.sendQue.Get(&writeList)上
	 *  因为无法继续执行doSend，对端将永远无法接收到完整的包
	*/
	for {
		if 0 != timeout {
			if time.Now().Unix() >= timeout {
				return ErrSendTimeout
			}			
		}

		/* 超时设置的必要性
		 * 如果对端一直不接收数据，那么doSend将永远阻塞在Flush上，即使本端调用Close也无法感知到(用finalSendTimeout>0调用Close,
		 * finalSendTimeout <=0 调用Close不存在这个问题，因为会调用CloseWrite使得阻塞的Write立即返回)		
		*/
		t := time.Now()
		deadline := t.Add(time.Second * 1)
		session.conn.SetWriteDeadline(deadline)

		err := writer.Flush()

		if 0 != timeout {
			if  time.Now().Unix() >= timeout { 
				//到达最后期限，直接退出
				return ErrSendTimeout
			}
		}

		if err != nil {

			if err.(net.Error).Timeout() {
				//write超时,在这里检测session是否被调用了Close,如果是退出循环
				if session.closed {
					break
				}
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

func sendThreadFunc(session *streamSocket) {
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

	var timeout int64

	for {

		var writeList [] interface{}

		closed := session.sendQue.Get(&writeList)

		if closed {
			if 0 >= session.finalSendTimeout {
				//关闭，丢弃所有待发送数据
				break
			} else {
				timeout = time.Now().Unix() + session.finalSendTimeout
			}
		} else {
			if session.sendTimeout > 0 {
				timeout = time.Now().Unix() + session.sendTimeout				
			}
		}
		
		errorOnWirte := false
		
		for i := range writeList {

			msg := writeList[i].(*ByteBuffer)
			buff,err := msg.GetBytes(0,msg.datasize)

			if err != nil {
				//Todo: 记录日志
				errorOnWirte = true
				break				
			}

			if err = writeToWriter(writer,buff); err != nil {
				//Todo: 记录日志
				errorOnWirte = true
				break
			}

		}

		if errorOnWirte {
			//Todo: 记录日志
			break
		}

		if 0 == writer.Buffered() && closed {
			//没有数据需要发送了,退出
			break
		}

		if err := doSend(session,writer,timeout); err != nil {
			//Todo: 记录日志
			fmt.Printf("error on doSend:%s\n",err.Error())
			break
		}

	}

}


func (this *streamSocket) Start() error {

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

	if this.onPacket == nil {
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
	session 			:= new(streamSocket)
	session.conn 		 = conn
	session.sendQue      = util.NewQueue()
	return session
}

func (this *streamSocket) GetUnderConn() interface{} {
	return this.conn
}




//简易sock代理服务,暂不支持UDP和认证
package main

import(
	"net"
	"sync/atomic"
	"encoding/binary"
	"fmt"
	"os"
	"github.com/sniperHW/kendynet"
	"time"
	codec "github.com/sniperHW/kendynet/example/codec/stream_socket"		
	"github.com/sniperHW/kendynet/socket/stream_socket/tcp"
)

const(
	CHECK_VERSION = 1
	PROCESS_REQUEST = 2
	ESTABLISHED = 3
)

const(
	SOCKS_VERSION_4 = 4
	SOCKS_VERSION_5 = 5
)

const(
	SOCKS5_AUTH_NONE = 0x00
	SOCKS5_AUTH = 0x02
	SOCKS5_AUTH_UNACCEPTABLE = 0xFF
)

const(
	SOCKS_CMD_CONNECT = 0x01
	SOCKS_CMD_BIND = 0x02
	SOCKS5_CMD_UDP = 0x03
)

const(
	SOCKS5_ATYP_IPV4 = 0x01
	SOCKS5_ATYP_DOMAINNAME = 0x03
	SOCKS5_ATYP_IPV6 = 0x04
)

const(
	SOCKS5_SUCCEEDED = 0x00
	SOCKS5_GENERAL_SOCKS_SERVER_FAILURE
	SOCKS5_CONNECTION_NOT_ALLOWED_BY_RULESET
	SOCKS5_NETWORK_UNREACHABLE
	SOCKS5_CONNECTION_REFUSED
	SOCKS5_TTL_EXPIRED
	SOCKS5_COMMAND_NOT_SUPPORTED
	SOCKS5_ADDRESS_TYPE_NOT_SUPPORTED
	SOCKS5_UNASSIGNED
)

type ProxySession struct{
	client     kendynet.StreamSession
	server     kendynet.StreamSession
	buff      *kendynet.ByteBuffer
	status     int
	closed     int32     
}

func (self *ProxySession) Close() {
	
	if !atomic.CompareAndSwapInt32(&self.closed,0,1) {
		return
	}

	self.client.Close("",5)
	self.client = nil
	if nil != self.server {
		self.server.Close("",0)
		self.server = nil
	}
}

func onServerEvent(event *kendynet.Event) {
	proxySession := event.Session.GetUserData().(*ProxySession)
	if event.EventType == kendynet.EventTypeError {
		proxySession.Close()
	} else {
		proxySession.client.SendMessage(event.Data.(kendynet.Message))
	}
}

func onClientEvent(event *kendynet.Event) {
	proxySession := event.Session.GetUserData().(*ProxySession)
	if event.EventType == kendynet.EventTypeError {
		proxySession.Close()
	} else {
		proxySession.ProcessClientMsg(event.Data.(kendynet.Message))
	}
}

func (self *ProxySession) ProcessClientMsg(msg kendynet.Message) {
	if self.status == ESTABLISHED {
		//连接已经建立，直接转发数据
		self.server.SendMessage(msg)
	} else {
		self.buff.AppendBytes(msg.Bytes())
		b := self.buff.Bytes()
		if len(b) >= 1 {
			if b[0] == SOCKS_VERSION_4 {
				self.ProcessSockV4()
			}else if b[0] == SOCKS_VERSION_5 {
				self.ProcessSockV5()
			}else {
				self.Close()
			}
		}
	}
}

func (self *ProxySession) ProcessSockV4() {
	if self.buff.Len() >= 9 {
		b := self.buff.Bytes()
		responseBuff := kendynet.NewByteBuffer(8)
		responseBuff.AppendUint64(0) //先填充一个8字节的0

		if b[8] != 0 {
			fmt.Printf("b[8] != 0\n")
			responseBuff.PutByte(1,byte(91))
			self.client.SendMessage(responseBuff)			
			self.Close()
			return
		}

		CMD := b[1]
		if CMD != SOCKS_CMD_CONNECT {
			fmt.Printf("CMD != SOCKS_CMD_CONNECT\n")
			//暂时不支持
			responseBuff.PutByte(1,byte(91))
			self.client.SendMessage(responseBuff)			
			self.Close()
			return
		}

		tcpAddr := net.TCPAddr{}
		tcpAddr.Port = int(binary.BigEndian.Uint16(b[2:4]))
		tcpAddr.IP = b[4:8]
		connector,err := tcp.NewConnector(tcpAddr.Network(),tcpAddr.String())
		if err != nil {
			fmt.Printf("NewConnector failed:%s\n",err.Error())
			return
		}
		session,err := connector.Dial(time.Second * 10)

		if err != nil {
			fmt.Printf("Dial failed:%s\n",err.Error())			
			return
		}

		session.SetReceiver(codec.NewRawReceiver(65535))
		session.SetEventCallBack(onServerEvent)
		session.SetUserData(self)
		self.server = session
		self.status = ESTABLISHED
		session.Start()
		responseBuff.PutByte(1,byte(90))
		err = self.client.SendMessage(responseBuff)
		if err != nil {
			fmt.Printf("send responseBuff to client failed:%s\n",err.Error())
		}
	} else {
		fmt.Printf("len < 9\n")
	}
}

func (self *ProxySession) ProcessSockV5() {
	if self.status == CHECK_VERSION {
		if self.buff.Len() >= 2 {
			b := self.buff.Bytes()
			VER := b[0]
			if VER != SOCKS_VERSION_5 {
				fmt.Printf("un support socks version:%d\n",int(VER))
				self.Close()
				return
			}

			NMETHODS := b[1]
			METHOD   := SOCKS5_AUTH_UNACCEPTABLE

			if self.buff.Len() >= uint64(2 + NMETHODS) {
				for i := 2; i < 2 + int(NMETHODS); i++ {
					if b[i] == SOCKS5_AUTH_NONE {
						METHOD = SOCKS5_AUTH_NONE
					}
				}
			}

			self.buff.Reset()

			responseBuff := kendynet.NewByteBuffer(2) //make([] byte,2)
			responseBuff.AppendByte(VER)
			responseBuff.AppendByte(byte(METHOD))
			self.client.SendMessage(responseBuff)

			if METHOD != SOCKS5_AUTH_NONE {
				fmt.Printf("un support auth\n")
				self.Close()
				return
			}else {
				self.status = PROCESS_REQUEST
			}
		}
	}else if self.status == PROCESS_REQUEST {
		if self.buff.Len() >= 4 {

			responseBuff := kendynet.NewByteBuffer(12)

			b := self.buff.Bytes()
			CMD := b[1]
			responseBuff.AppendByte(b[0])

			if CMD != SOCKS_CMD_CONNECT {	
				responseBuff.AppendByte(byte(SOCKS5_COMMAND_NOT_SUPPORTED))
				self.client.SendMessage(responseBuff)
				fmt.Printf("un support cmd\n")					
				self.Close()
				return
			}

			ATYP := b[3]

			addrSize := 0

			if ATYP == SOCKS5_ATYP_IPV4 {
				addrSize = 4
				if self.buff.Len() < uint64(6+addrSize) {
					return
				}
			}else if ATYP == SOCKS5_ATYP_IPV6 {
				addrSize = 8
				if self.buff.Len() < uint64(6+addrSize) {
					return
				}
			}else {	
				responseBuff.AppendByte(byte(SOCKS5_ADDRESS_TYPE_NOT_SUPPORTED))
				self.client.SendMessage(responseBuff)				
				fmt.Printf("un support address type\n")					
				self.Close()
				return					
			}

			tcpAddr := net.TCPAddr{}
			tcpAddr.Port = int(binary.BigEndian.Uint16(b[4+addrSize:6+addrSize]))
			tcpAddr.IP = b[4:4+addrSize]


			connector,err := tcp.NewConnector(tcpAddr.Network(),tcpAddr.String())
			if err != nil {
				fmt.Printf("NewConnector failed:%s\n",err.Error())
				return
			}
			session,err := connector.Dial(time.Second * 10)

			if err != nil {
				fmt.Printf("Dial failed:%s\n",err.Error())			
				return
			}
			session.SetReceiver(codec.NewRawReceiver(65535))
			session.SetUserData(self)
			self.server = session
			self.status = ESTABLISHED
			session.Start(onServerEvent)
			responseBuff.AppendByte(byte(SOCKS5_SUCCEEDED))
			responseBuff.AppendByte(byte(0))
			responseBuff.AppendBytes(b[3:6+addrSize])
			err = self.client.SendMessage(responseBuff)
			if err != nil {
				fmt.Printf("send responseBuff to client failed:%s\n",err.Error())
			}
			fmt.Printf("ESTABLISHED\n")
		}
	}
}

func main() {

	if len(os.Args) < 2 {
		fmt.Printf("usage ./sock_proxy ip:port\n")
		return
	}
	service := os.Args[1]

	server,err := tcp.NewListener("tcp4",service)
	if server != nil {
		fmt.Printf("server running on:%s\n",service)
		err = server.Start(func(session kendynet.StreamSession) {
			proxySession := &ProxySession{status:CHECK_VERSION}
			proxySession.client = session
			proxySession.server = nil
			proxySession.buff = kendynet.NewByteBuffer(128)
			session.SetUserData(proxySession)			
			session.SetReceiver(codec.NewRawReceiver(65535))
			session.Start(SetEventCallBack)
		})
		if nil != err {
			fmt.Printf("TcpServer start failed %s\n",err)			
		}

	} else {
		fmt.Printf("NewTcpServer failed %s\n",err)
	}
}

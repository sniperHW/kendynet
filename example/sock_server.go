//简易sock代理服务,暂不支持UDP和认证
package main

import(
	"net"
	"fmt"
	"sync/atomic"
	"os"
	"encoding/binary"
	"kendynet-go/kendynet"
	"kendynet-go/packet"
	"kendynet-go/golog"		
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

var loger *golog.Logger = golog.New("sock_server")

type ProxySession struct{
	clientSession    *kendynet.StreamConn
	serverSession    *kendynet.StreamConn
	buff             *packet.ByteBuffer
	status           int
	closed           int32     
}

func (self *ProxySession) Close() {
	
	if !atomic.CompareAndSwapInt32(&self.closed,0,1) {
		return
	}	
	self.clientSession.Close(5)
	self.clientSession = nil
	if nil != self.serverSession {
		self.serverSession.Close()
		self.serverSession = nil
	}
}

func onServerMsg(session *kendynet.StreamConn,msg *packet.ByteBuffer,errno error) {
	proxySession := session.UD().(*ProxySession)
	if msg == nil {
		proxySession.Close()
	}else {
		proxySession.clientSession.Send(msg)
	}
}

func (self *ProxySession) ProcessSockV4() {
	if self.buff.Len() >= 9 {
		buff,_ := self.buff.RawBinary()
		responseBuff := make([]byte,8)

		if buff[8] != 0 {
			responseBuff[1] = byte(91)
			self.clientSession.Send(packet.NewBufferByBytes(responseBuff,8))			
			self.Close()
			return
		}

		CMD := buff[1]
		if CMD != SOCKS_CMD_CONNECT {
			//暂时不支持
			responseBuff[1] = byte(91)
			self.clientSession.Send(packet.NewBufferByBytes(responseBuff,8))			
			self.Close()
			return	
		}

		tcpAddr := net.TCPAddr{}
		tcpAddr.Port = int(binary.BigEndian.Uint16(buff[2:4]))
		tcpAddr.IP = buff[4:8]


		tcpClient,err := kendynet.NewTcpClientAddr(&tcpAddr,packet.NewRawUnPacker(65535))
		if nil != tcpClient {
			tcpClient.OnConnected(func (session *kendynet.StreamConn){
				fmt.Printf("Dial ok:%d\n",int(responseBuff[0]))
				self.serverSession = session
				self.serverSession.SetUD(self)
				self.status = ESTABLISHED
				responseBuff[1] = byte(90)
				self.clientSession.Send(packet.NewBufferByBytes(responseBuff,8))
			})
			_,err = tcpClient.Dial(onServerMsg)
		}

		if nil != err {
			fmt.Printf("DialTcp error,%s\n",err)
			responseBuff[1] = byte(92)
			self.clientSession.Send(packet.NewBufferByBytes(responseBuff,8))
			self.Close()			
		}
	}
}

func (self *ProxySession) ProcessSockV5() {
	if self.status == CHECK_VERSION {
		if self.buff.Len() >= 2 {
			buff,_ := self.buff.RawBinary()
			VER := buff[0]
			if VER != SOCKS_VERSION_5 {
				fmt.Printf("un support socks version:%d\n",int(VER))
				self.Close()
				return
			}

			NMETHODS := buff[1]
			METHOD   := SOCKS5_AUTH_UNACCEPTABLE

			if self.buff.Len() >= uint32(2 + NMETHODS) {
				for i := 2; i < 2 + int(NMETHODS); i++ {
					if buff[i] == SOCKS5_AUTH_NONE {
						METHOD = SOCKS5_AUTH_NONE
					}
				}
			}

			self.buff.Reset()

			responseBuff := make([] byte,2)

			responseBuff[0] = VER
			responseBuff[1] = byte(METHOD)

			self.clientSession.Send(packet.NewBufferByBytes(responseBuff,2))

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

			responseBuff := make([] byte,12)

			buff,_ := self.buff.RawBinary()
			CMD := buff[1]
			responseBuff[0] = buff[0]

			if CMD != SOCKS_CMD_CONNECT {	
				responseBuff[1] = SOCKS5_COMMAND_NOT_SUPPORTED
				self.clientSession.Send(packet.NewBufferByBytes(responseBuff,2))
				fmt.Printf("un support cmd\n")					
				self.Close()
				return
			}

			ATYP := buff[3]

			addrSize := 0

			if ATYP == SOCKS5_ATYP_IPV4 {
				addrSize = 4
				if self.buff.Len() < uint32(6+addrSize) {
					return
				}
			}else if ATYP == SOCKS5_ATYP_IPV6 {
				addrSize = 8
				if self.buff.Len() < uint32(6+addrSize) {
					return
				}
			}else {
				responseBuff[1] = SOCKS5_ADDRESS_TYPE_NOT_SUPPORTED
				self.clientSession.Send(packet.NewBufferByBytes(responseBuff,2))					
				fmt.Printf("un support address type\n")					
				self.Close()
				return					
			}

			tcpAddr := net.TCPAddr{}
			tcpAddr.Port = int(binary.BigEndian.Uint16(buff[4+addrSize:6+addrSize]))
			tcpAddr.IP = buff[4:4+addrSize]

			tcpClient,err := kendynet.NewTcpClientAddr(&tcpAddr,packet.NewRawUnPacker(65535))

			if nil != tcpClient {
				tcpClient.OnConnected(func (session *kendynet.StreamConn){
					self.serverSession = session
					self.serverSession.SetUD(self)
					self.status = ESTABLISHED
					responseBuff[1] = SOCKS5_SUCCEEDED
					copy(responseBuff[3:],buff[3:6+addrSize])
					self.clientSession.Send(packet.NewBufferByBytes(responseBuff,uint32(6+addrSize)))
				})				
				_,err = tcpClient.Dial(onServerMsg)
			}

			if nil != err {
				responseBuff[1] = SOCKS5_CONNECTION_REFUSED
				self.clientSession.Send(packet.NewBufferByBytes(responseBuff,2))
				self.Close()			
			}
		}
	}	
}

func (self *ProxySession) ProcessClientMsg(msg *packet.ByteBuffer) {
	if self.status == ESTABLISHED {
		self.serverSession.Send(msg)
	} else {
		self.buff.PutByteBuffer(msg)
		if self.buff.Len() >= 1 {
			buff,_ := self.buff.RawBinary()
			if buff[0] == SOCKS_VERSION_4 {
				self.ProcessSockV4()
			}else if buff[0] == SOCKS_VERSION_5 {
				self.ProcessSockV5()
			}else {
				self.Close()
			}
		}
	}
}


func onClientMsg(session *kendynet.StreamConn,msg *packet.ByteBuffer,errno error) {
	proxySession := session.UD().(*ProxySession)

	if msg == nil {
		fmt.Printf("client close\n")
		proxySession.Close()
	}else {
		proxySession.ProcessClientMsg(msg)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage ./sock_server ip:port\n")
		return
	}

	service := os.Args[1]

	tcpServer,err := kendynet.NewTcpServer(service,packet.NewRawUnPacker(4096))

	if nil != err {
		fmt.Printf(err.Error())
		return
	}

	tcpServer.OnNewClient(func (session *kendynet.StreamConn) bool {
		proxySession := &ProxySession{status:CHECK_VERSION}
		proxySession.clientSession = session
		proxySession.serverSession = nil
		proxySession.buff = packet.NewByteBuffer(1024)
		session.SetUD(proxySession)
		return true
	})

	loger.Infoln("server running on ",service)
	tcpServer.Start(onClientMsg)

}

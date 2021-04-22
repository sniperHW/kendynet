package aio

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go tool cover -html=coverage.out
import (
	"errors"
	"fmt"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/buffer"
	"github.com/stretchr/testify/assert"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

type encoder struct {
}

func (this *encoder) EnCode(o interface{}, b *buffer.Buffer) error {
	switch o.(type) {
	case string:
		b.AppendString(o.(string))
	case []byte:
		b.AppendBytes(o.([]byte))
	default:
		return errors.New("invaild o")
	}
	return nil
}

type errencoder struct {
}

func (this *errencoder) EnCode(o interface{}, b *buffer.Buffer) error {
	return errors.New("invaild o")
}

var aioService *SocketService

func init() {
	aioService = NewSocketService(nil)
}

func TestAioSocket(t *testing.T) {
	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		die := make(chan struct{})

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				} else {
					session := NewSocket(aioService, conn)
					session.SetRecvTimeout(time.Second * 1).SetSendTimeout(time.Second * 1)
					session.GetUnderConn()
					session.SetSendQueueSize(1000)
					session.SetEncoder(&encoder{})
					session.SetInBoundProcessor(&defaultInBoundProcessor{buffer: make([]byte, 4096)})
					session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
						fmt.Println("server close")
						close(die)
						fmt.Println("close die")
					})
					session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
						if err := s.Send(msg); nil != err {
							panic(err)
						}
					})
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewSocket(aioService, conn)

		session.SetUserData(1)
		assert.Equal(t, 1, session.GetUserData().(int))

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			fmt.Println("client close")
			assert.Equal(t, true, sess.IsClosed())
		})

		respChan := make(chan interface{})

		session.SetEncoder(&encoder{}).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
			respChan <- msg
		})

		if err := session.Send("hello"); nil != err {
			panic(err)
		}

		resp := <-respChan

		assert.Equal(t, resp.([]byte), []byte("hello"))

		<-die

		listener.Close()

	}
	fmt.Println("1")
	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				} else {
					session := NewSocket(aioService, conn).SetRecvTimeout(time.Second * 1)
					session.SetEncoder(&encoder{})
					session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
						s.Send(msg)
						s.Close(nil, time.Second)
					})
				}
			}
		}()
		fmt.Println("2")
		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewSocket(aioService, conn)

			respChan := make(chan interface{})

			session.SetEncoder(&encoder{})
			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				respChan <- msg
			})

			session.Send("hello")

			resp := <-respChan

			assert.Equal(t, resp.([]byte), []byte("hello"))
		}
		fmt.Println("3")
		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewSocket(aioService, conn)

			session.SetEncoder(&encoder{})

			assert.Equal(t, nil, session.Send("haha"))

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			})

			session.Close(nil, 0)

			err := session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {

			})

			assert.Equal(t, kendynet.ErrSocketClose, err)
		}
		fmt.Println("4")
		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			NewSocket(aioService, conn).SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				fmt.Println("reason", reason)
			})
			for i := 0; i < 3; i++ {
				time.Sleep(time.Second)
				runtime.GC()
			}
		}

		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewSocket(aioService, conn)

			die := make(chan struct{})

			session.SetEncoder(&errencoder{})

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				fmt.Println("close", reason)
				close(die)
			})

			session.Send("hello")

			<-die
		}

		listener.Close()
	}

	fmt.Println("here1")

	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		die := make(chan struct{})

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				} else {
					session := NewSocket(aioService, conn)
					session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
						close(die)
					})
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewSocket(aioService, conn)

		session.SetEncoder(&encoder{})

		session.Send("hello")

		session.Close(nil, time.Second)

		_ = <-die

		listener.Close()
	}

	fmt.Println("here2")

	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		serverdie := make(chan struct{})
		clientdie := make(chan struct{})

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				} else {
					session := NewSocket(aioService, conn)
					session.SetRecvTimeout(time.Second * 1)
					session.SetSendTimeout(time.Second * 1)
					session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
						fmt.Println("server close")
						close(serverdie)
					})
					session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
					})
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewSocket(aioService, conn)

		session.SetEncoder(&encoder{})

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			fmt.Println("client close")
			close(clientdie)
		})

		session.Send("hello")

		session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
		})

		go func() {
			for {
				if err := session.Send("hello"); nil != err {
					if err == kendynet.ErrSocketClose {
						break
					}
				}
			}
		}()

		go func() {
			time.Sleep(time.Second * 5)
			session.Close(nil, 1)
		}()

		<-clientdie
		<-serverdie

		listener.Close()
	}

}

func TestSendTimeout(t *testing.T) {

	//test send timeout
	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		var holdSession kendynet.StreamSession

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				} else {
					conn.(*net.TCPConn).SetReadBuffer(0)
					holdSession = NewSocket(aioService, conn)
					//不启动接收
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		conn.(*net.TCPConn).SetWriteBuffer(0)
		session := NewSocket(aioService, conn)

		die := make(chan struct{})

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			close(die)
		})

		session.SetEncoder(&encoder{})

		session.SetSendTimeout(time.Second)

		//session.SetSendQueueSize(1)

		session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
		})

		fmt.Println("--------------------")
		for {
			err := session.Send(strings.Repeat("a", 65536))
			//fmt.Println(err)
			if nil != err && err != kendynet.ErrSendQueFull {
				fmt.Println("break here")
				break
			}
		}
		<-die

		holdSession.Close(nil, 0)

		listener.Close()
	}

}

func TestShutDownRead(t *testing.T) {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

	listener, _ := net.ListenTCP("tcp", tcpAddr)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			} else {
				session := NewSocket(aioService, conn)
				session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
					fmt.Println("server close")
				})
				session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				})
			}
		}
	}()

	dialer := &net.Dialer{}
	conn, _ := dialer.Dial("tcp", "localhost:8110")
	session := NewSocket(aioService, conn)

	die := make(chan struct{})

	session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
		fmt.Println("client close", reason)
		close(die)
	})

	session.SetEncoder(&encoder{})

	session.SetRecvTimeout(time.Second * 1)

	session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
	})

	fmt.Println("ShutdownRead")

	session.ShutdownRead()

	time.Sleep(time.Second * 2)

	session.Close(nil, 0)

	//session.Send(kendynet.NewByteBuffer("hello"))

	<-die

	listener.Close()

}

func TestFinal(t *testing.T) {
	aioService.Close()
}

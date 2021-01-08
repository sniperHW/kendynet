package aio

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go tool cover -html=coverage.out
import (
	"errors"
	"fmt"
	"github.com/sniperHW/kendynet"
	//"github.com/sniperHW/kendynet/message"
	//connector "github.com/sniperHW/kendynet/socket/connector/aio"
	//listener "github.com/sniperHW/kendynet/socket/listener/aio"
	"github.com/stretchr/testify/assert"
	"net"
	//"net/http"
	//"net/url"
	"runtime"
	"strings"
	"testing"
	"time"
)

type encoder struct {
}

func (this *encoder) EnCode(o interface{}) (kendynet.Message, error) {
	switch o.(type) {
	case *kendynet.ByteBuffer:
		return o.(*kendynet.ByteBuffer), nil
	default:
		return nil, errors.New("invaild o")
	}
}

var aioService *AioService

func init() {
	aioService = NewAioService(1, 1, 1, nil)
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
					session := NewAioSocket(aioService, conn)
					session.SetRecvTimeout(time.Second * 1)
					session.SetSendTimeout(time.Second * 1)
					session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
						fmt.Println("server close")
						close(die)
					})
					session.Start(func(event *kendynet.Event) {
						if event.EventType == kendynet.EventTypeError {
							event.Session.Close(event.Data.(error).Error(), 0)
							assert.Equal(t, session.SendMessage(kendynet.NewByteBuffer("hello")), kendynet.ErrSocketClose)
						} else {
							event.Session.SendMessage(event.Data.(kendynet.Message))
						}
					})
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewAioSocket(aioService, conn)

		session.SetUserData(1)
		assert.Equal(t, 1, session.GetUserData().(int))

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			fmt.Println("client close")
		})

		respChan := make(chan kendynet.Message)

		session.SetEncoder(&encoder{})

		session.Start(func(event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				event.Session.Close(event.Data.(error).Error(), 0)
				assert.Equal(t, true, session.IsClosed())
				session = nil
			} else {
				respChan <- event.Data.(kendynet.Message)
			}
		})

		assert.Equal(t, session.SendMessage(nil), kendynet.ErrInvaildBuff)

		session.Send(kendynet.NewByteBuffer("hello"))

		resp := <-respChan

		assert.Equal(t, resp.Bytes(), []byte("hello"))

		<-die

		listener.Close()

	}

	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				} else {
					session := NewAioSocket(aioService, conn)
					session.SetRecvTimeout(time.Second * 1)
					session.Start(func(event *kendynet.Event) {
						if event.EventType == kendynet.EventTypeError {
							event.Session.Close(event.Data.(error).Error(), 0)
						} else {
							event.Session.SendMessage(event.Data.(kendynet.Message))
							event.Session.Close("close", time.Second)
						}
					})
				}
			}
		}()

		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewAioSocket(aioService, conn)

			respChan := make(chan kendynet.Message)

			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					respChan <- event.Data.(kendynet.Message)
				}
			})

			session.SendMessage(kendynet.NewByteBuffer("hello"))

			resp := <-respChan

			assert.Equal(t, resp.Bytes(), []byte("hello"))
		}

		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewAioSocket(aioService, conn)

			assert.Equal(t, kendynet.ErrInvaildObject, session.Send(nil))

			assert.Equal(t, kendynet.ErrInvaildEncoder, session.Send("haha"))

			session.SetEncoder(&encoder{})

			assert.NotNil(t, session.Send("haha"))

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			})

			session.Close("none", 0)

			err := session.Start(func(event *kendynet.Event) {

			})

			assert.Equal(t, kendynet.ErrSocketClose, err)
		}

		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewAioSocket(aioService, conn)
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Println("reason", reason)
			})
			_ = session.LocalAddr()
			_ = session.RemoteAddr()
			session = nil
			for i := 0; i < 10; i++ {
				time.Sleep(time.Second)
				runtime.GC()
			}
		}

		listener.Close()
	}

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
					session := NewAioSocket(aioService, conn)
					session.Start(func(event *kendynet.Event) {
						if event.EventType == kendynet.EventTypeError {
							event.Session.Close(event.Data.(error).Error(), 0)
						} else {
							close(die)
						}
					})
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewAioSocket(aioService, conn)

		session.SetEncoder(&encoder{})

		session.Send(kendynet.NewByteBuffer("hello"))

		session.Close("none", time.Second)

		_ = <-die

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
					holdSession = NewAioSocket(aioService, conn)
					//不启动接收
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		conn.(*net.TCPConn).SetWriteBuffer(0)
		session := NewAioSocket(aioService, conn)

		die := make(chan struct{})

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			close(die)
		})

		session.SetEncoder(&encoder{})

		session.SetSendTimeout(time.Second)

		//session.SetSendQueueSize(1)

		session.Start(func(event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				assert.Equal(t, kendynet.ErrSendTimeout, event.Data.(error))
				event.Session.Close(event.Data.(error).Error(), 0)
			} else {

			}
		})

		for i := 0; i < 20; i++ {
			err := session.Send(kendynet.NewByteBuffer(strings.Repeat("a", 65536)))
			if nil != err {
				assert.Equal(t, kendynet.ErrSendQueFull, err)
			}
		}
		<-die

		holdSession.Close("none", 0)

		listener.Close()
	}

}
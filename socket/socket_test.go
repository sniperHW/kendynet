package socket

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go tool cover -html=coverage.out
import (
	"errors"
	"fmt"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/message"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/url"
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

type wsencoder struct {
}

func (this *wsencoder) EnCode(o interface{}) (kendynet.Message, error) {
	switch o.(type) {
	case string:
		return message.NewWSMessage(message.WSTextMessage, o.(string)), nil
	default:
		return nil, errors.New("invaild o")
	}
}

func TestWebSocket(t *testing.T) {
	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		upgrader := &gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}

		die := make(chan struct{})

		http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
			conn, _ := upgrader.Upgrade(w, r, nil)
			assert.NotNil(t, conn)
			session := NewWSSocket(conn)
			session.SetRecvTimeout(time.Second * 1)
			session.SetSendTimeout(time.Second * 1)
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				close(die)
			})
			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
					assert.Equal(t, session.SendMessage(message.NewWSMessage(message.WSTextMessage, "hello")), kendynet.ErrSocketClose)
				} else {
					event.Session.SendMessage(event.Data.(kendynet.Message))
				}
			})
		})

		go func() {
			http.Serve(listener, nil)
		}()

		u := url.URL{Scheme: "ws", Host: "localhost:8110", Path: "/echo"}
		dialer := gorilla.DefaultDialer

		conn, _, _ := dialer.Dial(u.String(), nil)
		assert.NotNil(t, conn)
		session := NewWSSocket(conn)

		respChan := make(chan kendynet.Message)

		session.SetReceiver(session.(*WebSocket).defaultReceiver())

		session.SetEncoder(&wsencoder{})

		session.Start(func(event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				event.Session.Close(event.Data.(error).Error(), 0)
			} else {
				respChan <- event.Data.(kendynet.Message)
			}
		})

		session.SetSendQueueSize(100)

		session.Send("hello")

		resp := <-respChan

		assert.Equal(t, resp.Bytes(), []byte("hello"))

		<-die

		listener.Close()
	}

	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		upgrader := &gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}

		http.HandleFunc("/echo1", func(w http.ResponseWriter, r *http.Request) {
			conn, _ := upgrader.Upgrade(w, r, nil)
			assert.NotNil(t, conn)
			session := NewWSSocket(conn)
			session.SetRecvTimeout(time.Second * 1)
			session.SetSendTimeout(time.Second * 1)

			session.(*WebSocket).SetPingHandler(func(appData string) error {
				fmt.Println(appData)
				session.SendMessage(message.NewWSMessage(message.WSPongMessage, "pong"))
				return nil
			})

			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					fmt.Println("err", event.Data.(error).Error())
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					event.Session.SendMessage(event.Data.(kendynet.Message))
					event.Session.Close("close", time.Second)
				}
			})
		})

		go func() {
			http.Serve(listener, nil)
		}()

		u := url.URL{Scheme: "ws", Host: "localhost:8110", Path: "/echo1"}
		dialer := gorilla.DefaultDialer

		{
			fmt.Println("0000")
			conn, _, _ := dialer.Dial(u.String(), nil)
			assert.NotNil(t, conn)
			session := NewWSSocket(conn)

			respChan := make(chan kendynet.Message)

			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
					respChan <- event.Data.(kendynet.Message)
				}
			})

			assert.Equal(t, session.SendMessage(nil), kendynet.ErrInvaildBuff)

			session.SendMessage(message.NewWSMessage(message.WSTextMessage, "hello"))

			//session.Close("none", time.Second)

			resp := <-respChan

			assert.Equal(t, resp.Bytes(), []byte("hello"))

		}

		{
			fmt.Println("1111")
			conn, _, _ := dialer.Dial(u.String(), nil)
			assert.NotNil(t, conn)
			session := NewWSSocket(conn)

			die := make(chan struct{})

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				close(die)
			})

			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {

				}
			})

			assert.Equal(t, session.SendMessage(nil), kendynet.ErrInvaildBuff)

			session.SendMessage(message.NewWSMessage(message.WSTextMessage, "hello"))

			session.Close("none", time.Second)

			<-die
		}

		{

			fmt.Println("test Ping Pong")

			//send ping
			conn, _, _ := dialer.Dial(u.String(), nil)
			assert.NotNil(t, conn)
			session := NewWSSocket(conn)

			die := make(chan struct{})

			//session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			//	close(die)
			//})

			session.(*WebSocket).SetPongHandler(func(appData string) error {
				fmt.Println(appData)
				close(die)
				return nil
			})

			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
					event.Session.Close(event.Data.(error).Error(), 0)
				} else {
				}
			})

			err := session.SendMessage(message.NewWSMessage(message.WSPingMessage, "ping"))

			if nil != err {
				fmt.Println(err)
			} else {
				fmt.Println("send ping ok")
			}

			//session.SendMessage(message.NewWSMessage(message.WSTextMessage, "hello"))

			//session.Close("none", time.Second)

			<-die
		}

		{
			//test wm close
			{

				fmt.Println("test wn close")

				//send ping
				conn, _, _ := dialer.Dial(u.String(), nil)
				assert.NotNil(t, conn)
				session := NewWSSocket(conn)

				die := make(chan struct{})

				session.Start(func(event *kendynet.Event) {
					if event.EventType == kendynet.EventTypeError {
						event.Session.Close(event.Data.(error).Error(), 0)
						close(die)
					} else {
					}
				})

				session.(*WebSocket).SendWSClose("wm close")

				<-die
			}

		}

		{
			//test gc
			conn, _, _ := dialer.Dial(u.String(), nil)
			assert.NotNil(t, conn)
			session := NewWSSocket(conn)
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
				fmt.Println("reason", reason)
			})
			session = nil
			for i := 0; i < 10; i++ {
				time.Sleep(time.Second)
				runtime.GC()
			}
		}

		listener.Close()
	}
}

func TestStreamSocket(t *testing.T) {
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
					session := NewStreamSocket(conn)
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
		session := NewStreamSocket(conn)

		session.SetUserData(1)
		assert.Equal(t, 1, session.GetUserData().(int))

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			fmt.Println("client close")
		})

		respChan := make(chan kendynet.Message)

		session.SetReceiver(session.(*StreamSocket).defaultReceiver())

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
					session := NewStreamSocket(conn)
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
			session := NewStreamSocket(conn)

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
			session := NewStreamSocket(conn)

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
			session := NewStreamSocket(conn)
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
					session := NewStreamSocket(conn)
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
		session := NewStreamSocket(conn)

		session.SetEncoder(&encoder{})

		session.Send(kendynet.NewByteBuffer("hello"))

		session.Close("none", time.Second)

		_ = <-die

		listener.Close()
	}
}

func TestSendTimeout(t *testing.T) {

	//web socket

	{

		tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

		listener, _ := net.ListenTCP("tcp", tcpAddr)

		upgrader := &gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}

		var holdSession kendynet.StreamSession

		die := make(chan struct{})

		http.HandleFunc("/echo2", func(w http.ResponseWriter, r *http.Request) {
			conn, _ := upgrader.Upgrade(w, r, nil)
			assert.NotNil(t, conn)
			holdSession = NewWSSocket(conn)
			conn.UnderlyingConn().(*net.TCPConn).SetReadBuffer(0)
		})

		go func() {
			http.Serve(listener, nil)
		}()

		u := url.URL{Scheme: "ws", Host: "localhost:8110", Path: "/echo2"}
		dialer := gorilla.DefaultDialer

		conn, _, _ := dialer.Dial(u.String(), nil)
		assert.NotNil(t, conn)
		session := NewWSSocket(conn)

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			close(die)
		})

		conn.UnderlyingConn().(*net.TCPConn).SetWriteBuffer(0)

		session.SetReceiver(session.(*WebSocket).defaultReceiver())

		session.SetEncoder(&wsencoder{})

		session.SetSendTimeout(time.Second)

		session.SetSendQueueSize(1)

		session.Start(func(event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				assert.Equal(t, kendynet.ErrSendTimeout, event.Data.(error))
				event.Session.Close(event.Data.(error).Error(), 0)
			} else {

			}
		})

		for i := 0; i < 20; i++ {
			err := session.SendMessage(message.NewWSMessage(message.WSTextMessage, strings.Repeat("a", 65536)))
			if nil != err {
				assert.Equal(t, kendynet.ErrSendQueFull, err)
			}
		}
		<-die

		holdSession.Close("none", 0)

		<-die

		listener.Close()
	}

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
					holdSession = NewStreamSocket(conn)
					//不启动接收
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		conn.(*net.TCPConn).SetWriteBuffer(0)
		session := NewStreamSocket(conn)

		die := make(chan struct{})

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason string) {
			close(die)
		})

		session.SetReceiver(session.(*StreamSocket).defaultReceiver())

		session.SetEncoder(&encoder{})

		session.SetSendTimeout(time.Second)

		session.SetSendQueueSize(1)

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

/*
func TestGC(t *testing.T) {
	_ = NewStreamSocket(nil)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		runtime.GC()
	}
}
*/

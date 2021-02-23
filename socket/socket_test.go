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

func TestSendTimeout(t *testing.T) {

	//web socket

	fmt.Println("websocket")

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

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			close(die)
		})

		conn.UnderlyingConn().(*net.TCPConn).SetWriteBuffer(0)

		session.SetInBoundProcessor(session.(*WebSocket).defaultInBoundProcessor())

		session.SetEncoder(&wsencoder{})

		session.SetSendTimeout(time.Second)

		session.SetSendQueueSize(1)

		session.SetErrorCallBack(func(sess kendynet.StreamSession, err error) {
			assert.Equal(t, kendynet.ErrSendTimeout, err)
			sess.Close(err, 0)
		})

		session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
		})

		go func() {
			for {
				err := session.SendMessage(message.NewWSMessage(message.WSTextMessage, strings.Repeat("a", 65536)))
				if nil != err && err != kendynet.ErrSendQueFull {
					break
				}
			}
		}()

		<-die

		holdSession.Close(nil, 0)

		<-die

		listener.Close()
	}

	//test send timeout
	fmt.Println("StreamSocket")
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

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			close(die)
		})

		session.SetInBoundProcessor(session.(*StreamSocket).defaultInBoundProcessor())

		session.SetEncoder(&encoder{})

		session.SetSendTimeout(time.Second)

		session.SetSendQueueSize(1)

		session.SetErrorCallBack(func(sess kendynet.StreamSession, err error) {
			assert.Equal(t, kendynet.ErrSendTimeout, err)
			sess.Close(err, 0)
		})

		session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
		})

		go func() {

			for {
				err := session.Send(kendynet.NewByteBuffer(strings.Repeat("a", 65536)))
				if nil != err && err != kendynet.ErrSendQueFull {
					break
				}
			}
		}()
		<-die

		holdSession.Close(nil, 0)

		listener.Close()
	}

}

func TestWebSocket(t *testing.T) {

	assert.Nil(t, NewWSSocket(nil))
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
			session.GetUnderConn()
			session.SetRecvTimeout(time.Second * 1)
			session.SetSendTimeout(time.Second * 1)
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				close(die)
			})

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				s.SendMessage(msg.(kendynet.Message))
			})

			assert.Equal(t, ErrInvaildWSMessage, session.SendMessage(kendynet.NewByteBuffer("hello")))

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

		session.SetEncoder(&wsencoder{}).SetSendQueueSize(100).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
			s.SendMessage(msg.(kendynet.Message))
			respChan <- msg.(kendynet.Message)
		})

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
			session.SetRecvTimeout(time.Second * 1).SetSendTimeout(time.Second * 1)

			session.(*WebSocket).SetPingHandler(func(appData string) error {
				fmt.Println(appData)
				session.SendMessage(message.NewWSMessage(message.WSPongMessage, "pong"))
				return nil
			})

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				s.SendMessage(msg.(kendynet.Message))
				s.Close(nil, time.Second)
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

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				respChan <- msg.(kendynet.Message)
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

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				close(die)
			})

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {

			})

			assert.Equal(t, session.SendMessage(nil), kendynet.ErrInvaildBuff)

			session.SendMessage(message.NewWSMessage(message.WSTextMessage, "hello"))

			session.Close(nil, time.Second)

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

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {

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

				session.SetErrorCallBack(func(sess kendynet.StreamSession, reason error) {
					close(die)
				}).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {

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
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				fmt.Println("reason", reason)
			})
			session = nil
			for i := 0; i < 2; i++ {
				time.Sleep(time.Second)
				runtime.GC()
			}
		}

		listener.Close()
	}
}

func TestStreamSocket(t *testing.T) {

	assert.Nil(t, NewStreamSocket(nil))

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
					session.GetUnderConn()
					session.SetRecvTimeout(time.Second * 1).SetSendTimeout(time.Second * 1)
					session.SetCloseCallBack(func(s kendynet.StreamSession, reason error) {
						fmt.Println("server close")
						close(die)
					})

					session.SetErrorCallBack(func(s kendynet.StreamSession, err error) {
						s.Close(err, 0)
						assert.Equal(t, s.SendMessage(kendynet.NewByteBuffer("hello")), kendynet.ErrSocketClose)
					}).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
						s.SendMessage(msg.(kendynet.Message))
					})
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewStreamSocket(conn)

		respChan := make(chan kendynet.Message)

		session.SetUserData(1)
		assert.Equal(t, 1, session.GetUserData().(int))

		session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			fmt.Println("client close")
		}).SetInBoundProcessor(session.(*StreamSocket).defaultInBoundProcessor()).SetEncoder(&encoder{})

		session.SetErrorCallBack(func(s kendynet.StreamSession, err error) {
			s.Close(err, 0)
			assert.Equal(t, true, s.IsClosed())
		}).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
			respChan <- msg.(kendynet.Message)
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
					NewStreamSocket(conn).SetRecvTimeout(time.Second * 1).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
						s.SendMessage(msg.(kendynet.Message))
						s.Close(nil, time.Second)
					})
				}
			}
		}()

		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewStreamSocket(conn)

			respChan := make(chan kendynet.Message)

			session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				respChan <- msg.(kendynet.Message)
			})

			session.SendMessage(kendynet.NewByteBuffer("hello"))

			resp := <-respChan

			assert.Equal(t, resp.Bytes(), []byte("hello"))
		}

		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewStreamSocket(conn)

			session.SetEncoder(&encoder{})

			assert.NotNil(t, session.Send("haha"))

			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			})

			session.Close(nil, 0)

			err := session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {

			})

			assert.Equal(t, kendynet.ErrSocketClose, err)
		}

		{
			dialer := &net.Dialer{}
			conn, _ := dialer.Dial("tcp", "localhost:8110")
			session := NewStreamSocket(conn)
			session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
				fmt.Println("reason", reason)
			})
			_ = session.LocalAddr()
			_ = session.RemoteAddr()
			session = nil
			for i := 0; i < 2; i++ {
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
					NewStreamSocket(conn).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
						close(die)
					})
				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewStreamSocket(conn)

		session.SetEncoder(&encoder{})

		session.Send(kendynet.NewByteBuffer("hello"))

		session.Close(nil, time.Second)

		_ = <-die

		listener.Close()
	}

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
					session := NewStreamSocket(conn)
					session.SetRecvTimeout(time.Second * 1)
					session.SetSendTimeout(time.Second * 1)
					session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
						close(serverdie)
					})
					session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
					})

				}
			}
		}()

		dialer := &net.Dialer{}
		conn, _ := dialer.Dial("tcp", "localhost:8110")
		session := NewStreamSocket(conn)

		session.SetEncoder(&encoder{}).SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
			close(clientdie)
		}).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
		})

		go func() {
			for {
				if err := session.Send(kendynet.NewByteBuffer("hello")); nil != err {
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

func TestShutDownRead(t *testing.T) {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", "localhost:8110")

	listener, _ := net.ListenTCP("tcp", tcpAddr)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			} else {
				NewStreamSocket(conn).SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
					fmt.Println("server close")
				}).BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
				})
			}
		}
	}()

	dialer := &net.Dialer{}
	conn, _ := dialer.Dial("tcp", "localhost:8110")
	session := NewStreamSocket(conn)

	die := make(chan struct{})

	session.SetCloseCallBack(func(sess kendynet.StreamSession, reason error) {
		fmt.Println("client close", reason)
		close(die)
	}).SetEncoder(&encoder{}).SetRecvTimeout(time.Second * 1)

	session.BeginRecv(func(s kendynet.StreamSession, msg interface{}) {
	})

	fmt.Println("ShutdownRead1")

	session.ShutdownRead()

	fmt.Println("ShutdownRead2")

	time.Sleep(time.Second * 2)

	session.Close(nil, 0)

	//session.Send(kendynet.NewByteBuffer("hello"))

	<-die

	listener.Close()

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

package socket

//go test -covermode=count -v -run=TestStreamSocket
import (
	"errors"
	gorilla "github.com/gorilla/websocket"
	"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/message"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/url"
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
					assert.Equal(t, session.Start(func(event *kendynet.Event) {}), kendynet.ErrSocketClose)
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

		assert.Equal(t, session.Start(func(event *kendynet.Event) {}), kendynet.ErrStarted)

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
			session.Start(func(event *kendynet.Event) {
				if event.EventType == kendynet.EventTypeError {
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

		resp := <-respChan

		assert.Equal(t, resp.Bytes(), []byte("hello"))

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
		})

		respChan := make(chan kendynet.Message)

		session.SetReceiver(session.(*StreamSocket).defaultReceiver())

		session.SetEncoder(&encoder{})

		session.Start(func(event *kendynet.Event) {
			if event.EventType == kendynet.EventTypeError {
				event.Session.Close(event.Data.(error).Error(), 0)
				assert.Equal(t, true, session.IsClosed())
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

		listener.Close()
	}

	//test eventWaiter
	/*{
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
					session.SetWaitMode(true)
					session.Start(func(event *kendynet.Event) {
						if event.EventType == kendynet.EventTypeError {
							event.Session.Close(event.Data.(error).Error(), 0)
						} else {
							event.Session.SendMessage(event.Data.(kendynet.Message))
							event.Session.Close("close", time.Second)
						}

						if nil != event.EventWaiter {
							event.EventWaiter.Notify()
						}

					})
				}
			}
		}()

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

		listener.Close()

	}*/

}

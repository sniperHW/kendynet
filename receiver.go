package kendynet

type Receiver interface {
	ReceiveAndUnpack(sess StreamSession) (interface{},error)
}
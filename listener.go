package kendynet

type Listener interface {
	Start(onNewClient func(StreamSession)) error
	Close()
}
package kendynet

type Server interface {
	Close()
	Start(func(StreamSession)) error
}
package kendynet

type EnCoder interface {
	EnCode(o interface{}) (*ByteBuffer,error)
}
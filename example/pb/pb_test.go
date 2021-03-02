package pb

//go test -covermode=count -v -coverprofile=coverage.out -run=.
//go tool cover -html=coverage.out
import (
	//"errors"
	//"fmt"
	"github.com/golang/protobuf/proto"
	//"github.com/sniperHW/kendynet"
	"github.com/sniperHW/kendynet/example/testproto"
	//"github.com/stretchr/testify/assert"
	//"net"
	//"runtime"
	//"strings"
	"testing"
	//"time"
)

func TestPB(t *testing.T) {
	Register(&testproto.Test{}, 1)
	o := &testproto.Test{}
	o.A = proto.String("hello")
	o.B = proto.Int32(17)
	b, _ := Encode(o, 4096)

	Decode(b.Bytes(), 0, uint64(len(b.Bytes())), 4096)
}
